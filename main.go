package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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

func NewJaegerExporter() (sdktrace.SpanExporter, error) {
	// Port details: https://www.jaegertracing.io/docs/getting-started/
	endpoint := os.Getenv("EXPORTER_ENDPOINT")

	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)))
	if err != nil {
		return nil, err
	}
	return exporter, nil
}
