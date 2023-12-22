package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/tj/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/bradfitz/gomemcache/memcache"

	_ "github.com/go-sql-driver/mysql"
)

var version string
var revision string
var build string

func main() {
	add(1, 2)
	_version()
}

func _version() {
	fmt.Println("ver: ", version, "rev: ", revision, "build: ", build)
}

func add(a, b int) int {
	return a + b
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

func NewTracerProvider(serviceName string) (*sdktrace.TracerProvider, func(), error) {
	exporter, err := NewJaegerExporter()
	if err != nil {
		return nil, nil, err
	}

	r := NewResource(serviceName, "1.0.0", "local")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := tp.ForceFlush(ctx); err != nil {
			log.Print(err)
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		if err := tp.Shutdown(ctx2); err != nil {
			log.Print(err)
		}
		cancel()
		cancel2()
	}
	return tp, cleanup, nil
}

func NewResource(serviceName string, version string, environment string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version),
		attribute.String("environment", environment),
	)
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
		// キャッシュになかった
		mc.Set(&memcache.Item{Key: "KEY", Value: "VALUE"})
	} else {
		// キャッシュあった
		val := it.Value
		fmt.Println(val)
	}
}
