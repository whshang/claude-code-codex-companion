package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"claude-code-codex-companion/internal/database"
)

func main() {
	var (
		oldDBPath = flag.String("old-db", "", "Path to old Python SQLite database (required)")
		newDBPath = flag.String("new-db", "./cccc.db", "Path to new Go SQLite database")
		help      = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *oldDBPath == "" {
		log.Fatal("Error: old-db parameter is required")
	}

	// 检查旧数据库文件是否存在
	if _, err := os.Stat(*oldDBPath); os.IsNotExist(err) {
		log.Fatalf("Error: old database file does not exist: %s", *oldDBPath)
	}

	fmt.Printf("Starting migration from Python to Go...\n")
	fmt.Printf("Old database: %s\n", *oldDBPath)
	fmt.Printf("New database: %s\n", *newDBPath)
	fmt.Printf("Note: API keys will be migrated as-is (no encryption/decryption)\n")

	// 执行迁移
	err := database.MigrateFromPython(*oldDBPath, *newDBPath, "")
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Printf("Migration completed successfully!\n")
	fmt.Printf("New database created at: %s\n", *newDBPath)
}

func showHelp() {
	fmt.Println("CCCC Database Migration Tool")
	fmt.Println("Migrate from Python Api-Conversion SQLite database to Go GORM database")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrate -old-db <old_db_path> [-new-db <new_db_path>]")
	fmt.Println()
	fmt.Println("Parameters:")
	fmt.Println("  -old-db string")
	fmt.Println("        Path to old Python SQLite database (required)")
	fmt.Println("  -new-db string")
	fmt.Println("        Path to new Go SQLite database (default \"./cccc.db\")")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  migrate -old-db ../Api-Conversion/data/channels.db -new-db ./cccc.db")
	fmt.Println()
	fmt.Println("Note:")
	fmt.Println("  - The old database should contain a 'channels' table from Api-Conversion")
	fmt.Println("  - The migration will skip the 'settings' table (admin authentication removed)")
	fmt.Println("  - API keys are migrated as-is (no encryption/decryption)")
	fmt.Println("  - All channels will be migrated with appropriate provider and format settings")
}
