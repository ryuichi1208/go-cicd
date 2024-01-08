package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/1Password/connect-sdk-go/connect"
	"github.com/Unleash/unleash-client-go/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hirochachacha/go-smb2"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/mmcdole/gofeed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tj/assert"
	nfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

var version string
var revision string
var build string

func main() {
	something()
}

func _version() {
	fmt.Println("ver: ", version, "rev: ", revision, "build: ", build)
}

func add(a, b int) int {
	return a + b + 1
}

// ファイルを開いて、読み込んで、書き込んで、閉じる
func openAndRead() {
	f, err := os.Open("test.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
}

func httpServer() {
	// インスタンスを作成
	e := echo.New()

	// ミドルウェアを設定
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", hello)

	// サーバーをポート番号1323で起動
	e.Logger.Fatal(e.Start(":1323"))
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

var tracer = otel.Tracer("otel-echo")

func initProvider(ctx context.Context) func() {
	fmt.Println("this1")
	// リソース情報（プロセス、ホスト、サービス名など）を設定
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOSType(),
		resource.WithProcessOwner(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(os.Args[2]),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	fmt.Println("this2")
	otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !ok {
		otelAgentAddr = "0.0.0.0:4317"
	}

	httpHeader := map[string]string{
		"X-Scope-OrgID": "1",
	}

	traceClient := otlptracehttp.NewClient(
		otlptracehttp.WithHeaders(httpHeader),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(otelAgentAddr),
		otlptracehttp.WithTimeout(5*time.Second),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{ // エクスポート失敗時にバッチ送信をリトライするための設定
			Enabled:         true,
			InitialInterval: 500 * time.Millisecond, // 最初の失敗後にリトライするまでの待ち時間
			MaxInterval:     5 * time.Second,        // 最大待ち時間
			MaxElapsedTime:  30 * time.Second,       // 最大経過時間
		}),
	)
	fmt.Println("this3")
	traceExp, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(
		traceExp,
		sdktrace.WithMaxQueueSize(5000),
		sdktrace.WithMaxExportBatchSize(512),
	)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	fmt.Println("this4")

	// リクエストヘッダーからトレースIDとスパンIDを取得するための設定
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	otel.SetTracerProvider(tracerProvider)

	fmt.Println("this5")
	return func() {
		cxt, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := traceExp.Shutdown(cxt); err != nil {
			otel.Handle(err)
		}
	}
}

func single() {
	fmt.Println("--- Start ---")
	eg, _ := errgroup.WithContext(context.Background())

	var g singleflight.Group
	for i := 0; i < 5; i++ {
		eg.Go(func() error {
			val, _, _ := g.Do("key", func() (interface{}, error) {
				fmt.Println("function called")
				time.Sleep(1 * time.Second)
				return "value", nil
			})

			fmt.Println(val)
			return nil
		})
	}

	_ = eg.Wait()
	fmt.Println("--- Done ---")
}

func TestPingMySql(t *testing.T) {
	db, err := sql.Open("mysql", "kazuhira:password@(172.17.0.2:3306)/practice")

	assert.Nil(t, err)
	assert.NotNil(t, db)

	defer db.Close()

	err = db.Ping()
	assert.Nil(t, err)
}

var (
	mc *memcache.Client
)

func memcached() {
	mc = memcache.New("127.0.0.1:11211")
	defer mc.Close()

	var it *memcache.Item
	it, err := mc.Get("KEY")
	if err != nil {
	} else {
		// キャッシュあった
		val := it.Value
		fmt.Println(val)
	}
}

func _redis() {
	var ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
		PoolSize: 1000,
	})

	rdb.Set(ctx, "mykey1", "hoge", 0)           // キー名 mykey1で文字列hogeをセット
	ret, err := rdb.Get(ctx, "mykey1").Result() // キー名mykey1を取得
	if err != nil {
		println("Error: ", err)
		return
	}

	println("Result: ", ret)
}

func zitter() {
	// リクエストを行う関数
	operation := func() error {
		resp, err := http.Get("https://example.com")
		if err != nil {
			// ネットワークエラーなど、再試行が必要な場合はエラーを返す
			return err
		}

		defer resp.Body.Close()

		// リクエストが成功した場合（HTTPステータスコード200）
		if resp.StatusCode == http.StatusOK {
			fmt.Println("リクエスト成功:", resp.Status)
			return nil
		}

		// サーバエラーなど、再試行が必要な場合はエラーを返す
		return fmt.Errorf("サーバエラー: %v", resp.Status)
	}

	// エクスポネンシャルバックオフの設定
	expBackOff := backoff.NewExponentialBackOff()
	expBackOff.MaxElapsedTime = 5 * time.Minute // 最大待ち時間を5分に設定

	// リトライ処理の実行
	err := backoff.Retry(operation, expBackOff)
	if err != nil {
		// 最終的にリトライが失敗した場合
		fmt.Println("リトライ失敗:", err)
		return
	}

	// 成功時の処理
	fmt.Println("リクエスト成功")
}

type GoStruct struct {
	A int
	B string
}

func _json() {
	stcData := GoStruct{A: 1, B: "bbb"}

	// Marshal関数でjsonエンコード
	// ->返り値jsonDataにはエンコード結果が[]byteの形で格納される
	jsonData, err := json.Marshal(stcData)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s\n", jsonData)
}

func onePass() {
	client, err := connect.NewClientFromEnvironment()
	item, err := client.GetItem("<item-uuid>", "<vault-uuid>")
	if err != nil {
		log.Fatal(item, err)
	}
}

func s3list() {
	creds := credentials.NewStaticCredentials("AWS_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String("ap-northeast-1")},
	)
	svc := s3.New(sess)

	fmt.Println(err, svc)
}

func s3listv2() {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		return
	}
	s3Client := s3v2.NewFromConfig(sdkConfig)
	count := 10
	fmt.Printf("Let's list up to %v buckets for your account.\n", count)
	result, err := s3Client.ListBuckets(context.TODO(), &s3v2.ListBucketsInput{})
	if err != nil {
		fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
		return
	}
	if len(result.Buckets) == 0 {
		fmt.Println("You don't have any buckets!")
	} else {
		if count > len(result.Buckets) {
			count = len(result.Buckets)
		}
		for _, bucket := range result.Buckets[:count] {
			fmt.Printf("\t%v\n", *bucket.Name)
		}
	}

}

func rssFeed() {
	feed, err := gofeed.NewParser().ParseURL("https://zenn.dev/spiegel/feed")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Println(feed.Title)
	fmt.Println(feed.FeedType, feed.FeedVersion)
	for _, item := range feed.Items {
		if item == nil {
			break
		}
		fmt.Println(item.Title)
		fmt.Println("\t->", item.Link)
		fmt.Println("\t->", item.PublishedParsed.Format(time.RFC3339))
	}
}

func aaa() {
	var aaa string
	fmt.Println(aaa)

	b := 1 + 2
	b = 10
	fmt.Println(b)
}

// メトリクスを記録する構造体
type MetricsMonitors struct {
	counter *prometheus.CounterVec
}

// MetricsMonitor の初期化メソッド
func NewMetricsMonitors() MetricsMonitors {
	monitors := MetricsMonitors{
		counter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myapp_request_total",
				Help: "The total number of requests with number param",
			},
			[]string{"odd_or_even"},
		),
	}
	prometheus.MustRegister(monitors.counter)
	return monitors
}

// メトリクスを記録するメソッド
func (mm *MetricsMonitors) record(number int) {
	if number%2 != 0 {
		m := mm.counter.WithLabelValues("odd")
		m.Inc()
	} else {
		m := mm.counter.WithLabelValues("even")
		m.Inc()
	}
}

func unleashCli() {
	unleash.Initialize(
		unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName("my-application"),
		unleash.WithUrl("http://unleash.herokuapp.com/api/"),
		unleash.WithCustomHeaders(http.Header{"Authorization": {"<API token>"}}),
	)

	// Note this will block until the default client is ready
	unleash.WaitForReady()
}

var (
	hostname = "mail.example.com"
	port     = 587
	username = "user@example.com"
	password = "password"
)

func __main() {
	from := "gopher@example.net"
	recipients := []string{"foo@example.com", "bar@example.com"}
	subject := "hello"
	body := "Hello World!\nHello Gopher!"

	auth := smtp.CRAMMD5Auth(username, password)
	msg := []byte(strings.ReplaceAll(fmt.Sprintf("To: %s\nSubject: %s\n\n%s", strings.Join(recipients, ","), subject, body), "\n", "\r\n"))
	if err := smtp.SendMail(fmt.Sprintf("%s:%d", hostname, port), auth, from, recipients, msg); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func ___main() {
	listener, err := net.Listen("tcp", ":0")
	panicOnErr(err, "starting TCP listener")
	fmt.Printf("Server running at %s\n", listener.Addr())
	mem := memfs.New()
	f, err := mem.Create("hello.txt")
	panicOnErr(err, "creating file")
	_, err = f.Write([]byte("hello world"))
	panicOnErr(err, "writing data")
	f.Close()
	handler := nfshelper.NewNullAuthHandler(mem)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1)
	panicOnErr(nfs.Serve(listener, cacheHelper), "serving nfs")
}

func panicOnErr(err error, desc ...interface{}) {
	if err == nil {
		return
	}
	log.Println(desc...)
	log.Panicln(err)
}

func smb() {
	conn, err := net.Dial("tcp", "SERVERNAME:445")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     "USERNAME",
			Password: "PASSWORD",
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		panic(err)
	}
	defer s.Logoff()

	names, err := s.ListSharenames()
	if err != nil {
		panic(err)
	}

	for _, name := range names {
		fmt.Println(name)
	}
}
