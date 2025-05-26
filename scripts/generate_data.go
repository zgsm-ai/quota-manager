package main

import (
	"fmt"
	"log"
	"math/rand"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"time"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("../config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	// Generate user data
	if err := generateUsers(db); err != nil {
		log.Fatalf("Failed to generate users: %v", err)
	}

	// Generate strategy data
	if err := generateStrategies(db); err != nil {
		log.Fatalf("Failed to generate strategies: %v", err)
	}

	fmt.Println("Data generation completed successfully!")
}

func generateUsers(db *database.DB) error {
	fmt.Println("Generating user data...")

	users := []models.UserInfo{
		{
			ID:             "user001",
			Name:           "John Doe",
			GithubUsername: "zhangsan",
			Email:          "zhangsan@example.com",
			Phone:          "13800138001",
			GithubStar:     "zgsm,openai/gpt-4,microsoft/vscode",
			VIP:            1,
			Org:            "org001",
			RegisterTime:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Hour * 2),
		},
		{
			ID:             "user002",
			Name:           "Jane Smith",
			GithubUsername: "lisi",
			Email:          "lisi@example.com",
			Phone:          "13800138002",
			GithubStar:     "zgsm,facebook/react",
			VIP:            2,
			Org:            "org001",
			RegisterTime:   time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Hour * 1),
		},
		{
			ID:             "user003",
			Name:           "Bob Wilson",
			GithubUsername: "wangwu",
			Email:          "wangwu@example.com",
			Phone:          "13800138003",
			GithubStar:     "google/tensorflow,pytorch/pytorch",
			VIP:            0,
			Org:            "org002",
			RegisterTime:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Hour * 3),
		},
		{
			ID:             "user004",
			Name:           "Alice Brown",
			GithubUsername: "zhaoliu",
			Email:          "zhaoliu@example.com",
			Phone:          "13800138004",
			GithubStar:     "zgsm,kubernetes/kubernetes",
			VIP:            3,
			Org:            "org002",
			RegisterTime:   time.Date(2022, 12, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Minute * 30),
		},
		{
			ID:             "user005",
			Name:           "Charlie Davis",
			GithubUsername: "sunqi",
			Email:          "sunqi@example.com",
			Phone:          "13800138005",
			GithubStar:     "",
			VIP:            0,
			Org:            "",
			RegisterTime:   time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Hour * 24),
		},
	}

	// Generate more random users
	for i := 6; i <= 20; i++ {
		user := models.UserInfo{
			ID:             fmt.Sprintf("user%03d", i),
			Name:           fmt.Sprintf("User%d", i),
			GithubUsername: fmt.Sprintf("user%d", i),
			Email:          fmt.Sprintf("user%d@example.com", i),
			Phone:          fmt.Sprintf("138%08d", rand.Intn(100000000)),
			VIP:            rand.Intn(4),
			RegisterTime:   time.Date(2023, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Duration(rand.Intn(168)) * time.Hour),
		}

		// Randomly assign organization
		if rand.Float32() < 0.7 {
			user.Org = fmt.Sprintf("org%03d", rand.Intn(5)+1)
		}

		// Randomly assign GitHub star
		if rand.Float32() < 0.6 {
			stars := []string{"zgsm", "openai/gpt-4", "microsoft/vscode", "facebook/react", "google/tensorflow"}
			starCount := rand.Intn(3) + 1
			selectedStars := make([]string, 0, starCount)
			for j := 0; j < starCount; j++ {
				selectedStars = append(selectedStars, stars[rand.Intn(len(stars))])
			}
			user.GithubStar = fmt.Sprintf("%v", selectedStars)
		}

		users = append(users, user)
	}

	// Batch insert user data
	for _, user := range users {
		if err := db.Create(&user).Error; err != nil {
			fmt.Printf("Warning: Failed to create user %s: %v\n", user.ID, err)
		}
	}

	fmt.Printf("Generated %d users\n", len(users))
	return nil
}

func generateStrategies(db *database.DB) error {
	fmt.Println("Generating strategy data...")

	strategies := []models.QuotaStrategy{
		{
			Name:         "recharge-star-everyday",
			Title:        "Daily Recharge for Starred Users",
			Type:         "periodic",
			Amount:       5,
			Model:        "claude-3-5-sonnet-latest",
			PeriodicExpr: "0 8 * * *", // Every day at 8 AM
			Condition:    `github-star("zgsm")`,
			Status:       true, // Enabled status
		},
		{
			Name:      "recharge-star-once",
			Title:     "One-time Recharge for Starred Users",
			Type:      "single",
			Amount:    20,
			Model:     "claude-3-5-sonnet-latest",
			Condition: `github-star("zgsm")`,
			Status:    true, // Enabled status
		},
		{
			Name:         "vip-daily-bonus",
			Title:        "Daily VIP User Reward",
			Type:         "periodic",
			Amount:       10,
			Model:        "gpt-4",
			PeriodicExpr: "0 9 * * *", // Every day at 9 AM
			Condition:    `is-vip(1)`,
			Status:       true, // Enabled status
		},
		{
			Name:      "new-user-welcome",
			Title:     "New User Welcome Reward",
			Type:      "single",
			Amount:    50,
			Model:     "gpt-3.5-turbo",
			Condition: `register-before("2024-06-01 00:00:00")`,
			Status:    false, // Disabled status (test)
		},
		{
			Name:         "org-weekly-bonus",
			Title:        "Organization User Weekly Reward",
			Type:         "periodic",
			Amount:       15,
			Model:        "claude-3-5-sonnet-latest",
			PeriodicExpr: "0 10 * * 1", // Every Monday at 10 AM
			Condition:    `belong-to("org001")`,
			Status:       true, // Enabled status
		},
		{
			Name:      "active-user-bonus",
			Title:     "Active User Reward",
			Type:      "single",
			Amount:    30,
			Model:     "gpt-4",
			Condition: `and(access-after("2024-05-01 00:00:00"), is-vip(2))`,
			Status:    true, // Enabled status
		},
		{
			Name:         "low-quota-refill",
			Title:        "Low Quota Automatic Refill",
			Type:         "periodic",
			Amount:       25,
			Model:        "gpt-3.5-turbo",
			PeriodicExpr: "0 */6 * * *", // Every 6 hours
			Condition:    `quota-le("gpt-3.5-turbo", 10)`,
			Status:       false, // Disabled status (requires configuration to enable)
		},
	}

	// Batch insert strategy data
	for _, strategy := range strategies {
		if err := db.Create(&strategy).Error; err != nil {
			fmt.Printf("Warning: Failed to create strategy %s: %v\n", strategy.Name, err)
		} else {
			fmt.Printf("Created strategy: %s (status: %t)\n", strategy.Name, strategy.Status)
		}
	}

	fmt.Printf("Generated %d strategies\n", len(strategies))
	return nil
}
