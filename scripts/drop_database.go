package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func main() {
	host := envOrDefault("CH_HOST", "localhost")
	port := envOrDefault("CH_PORT", "9000")
	user := envOrDefault("CH_USER", "default")
	password := os.Getenv("CH_PASSWORD")
	database := envOrDefault("CH_DATABASE", "default")

	initDir, err := resolveInitDir()
	if err != nil {
		log.Fatalf("resolve init SQL dir: %v", err)
	}

	conn, err := openConn(host, port, user, password, "default")
	if err != nil {
		log.Fatalf("open clickhouse: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("ping clickhouse: %v", err)
	}

	log.Printf("dropping database %q", database)
	if err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdent(database))); err != nil {
		log.Fatalf("drop database: %v", err)
	}

	log.Printf("creating database %q", database)
	if err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdent(database))); err != nil {
		log.Fatalf("create database: %v", err)
	}

	if err := conn.Close(); err != nil {
		log.Fatalf("close admin connection: %v", err)
	}

	conn, err = openConn(host, port, user, password, database)
	if err != nil {
		log.Fatalf("reopen clickhouse: %v", err)
	}
	defer conn.Close()

	if err := applyInitSQL(ctx, conn, initDir); err != nil {
		log.Fatalf("apply init SQL: %v", err)
	}

	log.Printf("database %q reset complete; schema recreated from %s", database, initDir)
	log.Println("run /app/jalali-seed (or restart the container) before using the API")
}

func openConn(host, port, user, password, database string) (driver.Conn, error) {
	return ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port)},
		Auth: ch.Auth{
			Database: database,
			Username: user,
			Password: password,
		},
		DialTimeout: 5 * time.Second,
	})
}

func resolveInitDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("INIT_SQL_DIR")); dir != "" {
		return dir, nil
	}

	candidates := []string{
		"/app/clickhouse/init",
		"../clickhouse/init",
		"clickhouse/init",
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("init SQL directory not found; set INIT_SQL_DIR")
}

func applyInitSQL(ctx context.Context, conn driver.Conn, initDir string) error {
	entries, err := os.ReadDir(initDir)
	if err != nil {
		return fmt.Errorf("read init dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		path := filepath.Join(initDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		statements := splitSQLStatements(string(content))
		if len(statements) == 0 {
			log.Printf("skipped %s (no executable statements)", name)
			continue
		}

		for _, statement := range statements {
			if err := conn.Exec(ctx, statement); err != nil {
				return fmt.Errorf("execute %s: %w\nstatement: %s", name, err, statement)
			}
		}
		log.Printf("applied %s (%d statements)", name, len(statements))
	}

	return nil
}

func splitSQLStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	var statements []string
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" || isCommentOnly(statement) {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}

func isCommentOnly(statement string) bool {
	for _, line := range strings.Split(statement, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "--") {
			return false
		}
	}
	return true
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
