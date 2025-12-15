package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"llm_gateway/internal/config"
	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"

	"github.com/google/uuid"
)

func main() {
	fmt.Println("LLM Gateway - Bootstrap Admin Initialization")
	fmt.Println("=" + string(make([]byte, 48)))

	// Load configuration (primarily for database connection)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Get bootstrap credentials from environment
	email := os.Getenv("ADMIN_BOOTSTRAP_EMAIL")
	password := os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")

	if email == "" || password == "" {
		fmt.Fprintf(os.Stderr, "ERROR: ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD must be set\n")
		os.Exit(1)
	}

	// Validate email format (basic check)
	if !isValidEmail(email) {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid email format: %s\n", email)
		os.Exit(1)
	}

	// Validate password strength (basic check)
	if len(password) < 8 {
		fmt.Fprintf(os.Stderr, "ERROR: Password must be at least 8 characters long\n")
		os.Exit(1)
	}

	// Connect to database
	fmt.Println("Connecting to database...")
	dbConfig := storage.DBConfig{
		DSN:             cfg.Database.URL,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
		APIKeyCacheSize: 10, // Minimal cache for init tool
		APIKeyCacheTTL:  5 * time.Minute,
		ModelCacheSize:  10,
		ModelCacheTTL:   5 * time.Minute,
	}

	db, err := storage.NewDB(dbConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("Database connection established")

	// Create repository
	repo := storage.NewAdminUserRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if any admin users already exist
	fmt.Println("Checking for existing admin users...")
	existingUsers, err := repo.List(ctx, false) // Get all users, not just enabled
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to check existing users: %v\n", err)
		os.Exit(1)
	}

	if len(existingUsers) > 0 {
		fmt.Printf("INFO: Found %d existing admin user(s). Bootstrap not needed.\n", len(existingUsers))
		fmt.Println("Existing users:")
		for _, user := range existingUsers {
			status := "enabled"
			if !user.Enabled {
				status = "disabled"
			}
			fmt.Printf("  - %s (%s) - Roles: %v\n", user.Email, status, user.Roles)
		}
		fmt.Println("\nExiting successfully (no action taken)")
		os.Exit(0)
	}

	// Check if user with this email already exists (edge case)
	existingUser, err := repo.GetByEmail(ctx, email)
	if err != nil && err != storage.ErrAdminUserNotFound {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to check for existing user: %v\n", err)
		os.Exit(1)
	}

	if existingUser != nil {
		fmt.Printf("INFO: Admin user with email %s already exists\n", email)
		fmt.Println("Exiting successfully (no action taken)")
		os.Exit(0)
	}

	// Hash password using Argon2
	fmt.Println("Hashing password using Argon2...")
	passwordHash, err := utils.HashPasswordArgon2(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// Create bootstrap admin user
	fmt.Printf("Creating bootstrap admin user: %s\n", email)
	adminUser := &models.AdminUser{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		Roles:        []string{"admin"}, // Full admin role
		Enabled:      true,
	}

	if err := repo.Create(ctx, adminUser); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create admin user: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + string(make([]byte, 50)))
	fmt.Println("SUCCESS: Bootstrap admin user created successfully!")
	fmt.Println(string(make([]byte, 50)))
	fmt.Printf("Email: %s\n", adminUser.Email)
	fmt.Printf("ID: %s\n", adminUser.ID)
	fmt.Printf("Roles: %v\n", adminUser.Roles)
	fmt.Printf("Created: %s\n", adminUser.CreatedAt.Format(time.RFC3339))
	fmt.Println("\nYou can now log in to the admin panel with these credentials.")
	fmt.Println("IMPORTANT: Store these credentials securely and consider changing the password after first login.")
	fmt.Println("\nFor security, you should now:")
	fmt.Println("1. Remove ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD from your environment")
	fmt.Println("2. Create additional admin users through the API if needed")
	fmt.Println("3. Consider disabling or rotating this initial admin account")
}

// isValidEmail performs a basic email validation
func isValidEmail(email string) bool {
	// Very basic check - just ensure it has @ and a domain
	if len(email) < 3 {
		return false
	}

	atCount := 0
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atCount++
			atIndex = i
		}
	}

	// Must have exactly one @ and it can't be at start or end
	if atCount != 1 || atIndex == 0 || atIndex == len(email)-1 {
		return false
	}

	// Must have at least one character after @
	if atIndex == len(email)-1 {
		return false
	}

	return true
}
