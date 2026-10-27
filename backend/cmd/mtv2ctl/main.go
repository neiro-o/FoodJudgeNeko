package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/models"
	"mtv2/backend/utils"
	"os"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mtv2ctl check-config|admin|init-mongo|init-es")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "mtv2ctl:", err)
		os.Exit(1)
	}
}
func run(cmd string) error {
	if cmd == "init-mongo" || cmd == "init-es" {
		return syscall.Exec("/app/mtv2ctl-legacy", []string{"mtv2ctl-legacy", cmd}, os.Environ())
	}
	if cmd != "admin" && cmd != "check-config" {
		return fmt.Errorf("unknown command")
	}
	path := os.Getenv("MTV2_CONFIG")
	if path == "" {
		path = "config.yml"
	}
	if err := config.LoadConfig(path); err != nil {
		return err
	}
	os.Setenv("MTV2_SKIP_STARTUP_INDEXES", "true")
	if err := database.Connect(); err != nil {
		return err
	}
	defer database.Disconnect()
	if cmd == "check-config" {
		if err := database.ConnectRedis(); err != nil {
			return err
		}
		defer database.DisconnectRedis()
		if err := database.ConnectElasticsearch(); err != nil {
			return err
		}
		return nil
	}
	name, email, password := os.Getenv("MTV2_ADMIN_USERNAME"), os.Getenv("MTV2_ADMIN_EMAIL"), os.Getenv("MTV2_ADMIN_PASSWORD")
	if name == "" || email == "" || len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("username, email and password of 12-72 bytes required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := database.Accounts.FindOne(ctx, bson.M{"$or": bson.A{bson.M{"username": name}, bson.M{"email": email}}}).Err()
	if err == nil {
		return fmt.Errorf("account already exists")
	}
	if err != mongo.ErrNoDocuments {
		return err
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = database.Accounts.InsertOne(ctx, models.Account{Username: name, Email: email, Password: hash, IsAdmin: true, CreatedAt: time.Now()})
	return err
}
