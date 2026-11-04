package main

import (
	"context"
	"fmt"
	"log"
	"mtv2/backend/bootstrap"
	"mtv2/backend/config"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: mtv2ctl init-mongo | init-es | admin | check-config")
	}
	if err := config.LoadConfig(config.Path()); err != nil {
		log.Fatal("Cannot load configuration")
	}
	if err := config.AppConfig.Validate(); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var err error
	switch os.Args[1] {
	case "check-config":
	case "init-mongo":
		err = bootstrap.InitMongo(ctx)
	case "init-es":
		err = bootstrap.InitES(ctx)
	case "admin":
		err = bootstrap.CreateAdmin(ctx, os.Getenv("MTV2_ADMIN_USERNAME"), os.Getenv("MTV2_ADMIN_EMAIL"), os.Getenv("MTV2_ADMIN_PASSWORD"))
	default:
		log.Fatal("unknown command")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(os.Args[1] + ": OK")
}
