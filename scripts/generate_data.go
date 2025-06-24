package main

import (
	"fmt"
	"log"
	"math/rand"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"strings"
	"time"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("../config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.NewDB(cfg)
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

	// Generate quota data
	if err := generateQuotas(db); err != nil {
		log.Fatalf("Failed to generate quotas: %v", err)
	}

	// Generate audit data
	if err := generateAudits(db); err != nil {
		log.Fatalf("Failed to generate audits: %v", err)
	}

	fmt.Println("Data generation completed successfully!")
}

func generateUsers(db *database.DB) error {
	fmt.Println("Generating user data...")

	users := []models.UserInfo{
		{
			ID:               "85054712",
			CreatedAt:        time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:        time.Now().Add(-1 * time.Hour),
			AccessTime:       time.Now().Add(-1 * time.Hour),
			Name:             "stoneHeartNew",
			GithubID:         "stoneheartnew",
			GithubName:       "stoneHeartNew",
			VIP:              1,
			Phone:            "13800138000",
			Email:            "2232078249@qq.com",
			Password:         "",
			Company:          "Sangfor",
			Location:         "Shenzhen",
			UserCode:         "EMP85054712",
			ExternalAccounts: "",
			EmployeeNumber:   "SF85054712",
			GithubStar:       "zgsm-ai.zgsm,zgsm-ai.casdoor,RooCodeInc.Roo-Code",
			Devices:          "{}",
		},
		{
			ID:               "12345678",
			CreatedAt:        time.Now().Add(-60 * 24 * time.Hour),
			UpdatedAt:        time.Now().Add(-2 * time.Hour),
			AccessTime:       time.Now().Add(-2 * time.Hour),
			Name:             "testUser1",
			GithubID:         "testuser1",
			GithubName:       "testuser1",
			VIP:              0,
			Phone:            "13900139001",
			Email:            "test1@example.com",
			Password:         "",
			Company:          "TestOrg1",
			Location:         "Beijing",
			UserCode:         "TEST001",
			ExternalAccounts: "",
			EmployeeNumber:   "T12345678",
			GithubStar:       "openai.gpt-4,microsoft.vscode",
			Devices:          "{}",
		},
		{
			ID:               "87654321",
			CreatedAt:        time.Now().Add(-90 * 24 * time.Hour),
			UpdatedAt:        time.Now().Add(-3 * time.Hour),
			AccessTime:       time.Now().Add(-3 * time.Hour),
			Name:             "testUser2",
			GithubID:         "testuser2",
			GithubName:       "testuser2",
			VIP:              2,
			Phone:            "13900139002",
			Email:            "test2@example.com",
			Password:         "",
			Company:          "Enterprise",
			Location:         "Shanghai",
			UserCode:         "ENT001",
			ExternalAccounts: "",
			EmployeeNumber:   "E87654321",
			GithubStar:       "zgsm-ai.zgsm,facebook.react,google.tensorflow",
			Devices:          "{}",
		},
	}

	// Generate more random users
	for i := 6; i <= 20; i++ {
		// Define possible GitHub project list
		possibleStars := []string{
			"zgsm-ai.zgsm",
			"zgsm-ai.casdoor",
			"RooCodeInc.Roo-Code",
			"openai.gpt-4",
			"microsoft.vscode",
			"facebook.react",
			"google.tensorflow",
			"kubernetes.kubernetes",
			"docker.docker",
			"golang.go",
		}

		// Randomly select several projects
		numStars := rand.Intn(4) + 1 // 1-4 projects
		selectedIndices := rand.Perm(len(possibleStars))[:numStars]
		var selectedStars []string
		for _, idx := range selectedIndices {
			selectedStars = append(selectedStars, possibleStars[idx])
		}

		user := models.UserInfo{
			ID:               fmt.Sprintf("user%03d", i),
			CreatedAt:        time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour),
			UpdatedAt:        time.Now().Add(-time.Duration(rand.Intn(24)) * time.Hour),
			AccessTime:       time.Now().Add(-time.Duration(rand.Intn(24)) * time.Hour),
			Name:             fmt.Sprintf("User%d", i),
			GithubID:         fmt.Sprintf("user%d", i),
			GithubName:       fmt.Sprintf("user%d", i),
			VIP:              rand.Intn(4),
			Phone:            fmt.Sprintf("138%08d", rand.Intn(100000000)),
			Email:            fmt.Sprintf("user%d@example.com", i),
			Password:         "",
			Company:          fmt.Sprintf("org%03d", rand.Intn(5)+1),
			Location:         fmt.Sprintf("%d, %d", rand.Intn(90)+10, rand.Intn(90)+10),
			UserCode:         fmt.Sprintf("EMP%08d", rand.Intn(100000000)),
			ExternalAccounts: "",
			EmployeeNumber:   fmt.Sprintf("E%08d", rand.Intn(100000000)),
			GithubStar:       strings.Join(selectedStars, ","),
			Devices:          "{}",
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

	// Create users in auth database
	for _, user := range users {
		if err := db.AuthDB.Create(&user).Error; err != nil {
			log.Fatalf("Failed to create user %s: %v", user.ID, err)
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
			log.Printf("Warning: Failed to create strategy %s: %v", strategy.Name, err)
		} else {
			log.Printf("Created strategy: %s (status: %t)", strategy.Name, strategy.Status)
		}
	}

	fmt.Printf("Generated %d strategies\n", len(strategies))
	return nil
}

func generateQuotas(db *database.DB) error {
	fmt.Println("Generating quota data...")

	quotas := []models.Quota{
		{
			UserID:     "user001",
			Amount:     100,
			ExpiryDate: time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusValid,
		},
		{
			UserID:     "user001",
			Amount:     50,
			ExpiryDate: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusValid,
		},
		{
			UserID:     "user002",
			Amount:     75,
			ExpiryDate: time.Date(2025, 7, 31, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusValid,
		},
		{
			UserID:     "user003",
			Amount:     25,
			ExpiryDate: time.Date(2025, 5, 31, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusValid,
		},
		{
			UserID:     "user004",
			Amount:     200,
			ExpiryDate: time.Date(2025, 8, 31, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusValid,
		},
		{
			UserID:     "user005",
			Amount:     10,
			ExpiryDate: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			Status:     models.StatusExpired,
		},
	}

	// Generate more random quotas for other users
	for i := 6; i <= 20; i++ {
		userID := fmt.Sprintf("user%03d", i)

		// Generate 1-3 quota records per user
		quotaCount := rand.Intn(3) + 1
		for j := 0; j < quotaCount; j++ {
			quota := models.Quota{
				UserID:     userID,
				Amount:     rand.Intn(150) + 10, // 10-160
				ExpiryDate: time.Date(2025, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 23, 59, 59, 0, time.UTC),
				Status:     models.StatusValid,
			}

			// Randomly make some quotas expired
			if rand.Float32() < 0.1 {
				quota.Status = models.StatusExpired
				quota.ExpiryDate = time.Date(2024, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 23, 59, 59, 0, time.UTC)
			}

			quotas = append(quotas, quota)
		}
	}

	// Batch insert quota data
	for _, quota := range quotas {
		if err := db.Create(&quota).Error; err != nil {
			log.Printf("Warning: Failed to create quota for %s: %v", quota.UserID, err)
		}
	}

	fmt.Printf("Generated %d quota records\n", len(quotas))
	return nil
}

func generateAudits(db *database.DB) error {
	fmt.Println("Generating audit data...")

	audits := []models.QuotaAudit{
		{
			UserID:       "user001",
			Amount:       100,
			Operation:    models.OperationRecharge,
			StrategyName: "recharge-star-everyday",
			ExpiryDate:   time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC),
		},
		{
			UserID:       "user001",
			Amount:       50,
			Operation:    models.OperationRecharge,
			StrategyName: "vip-daily-bonus",
			ExpiryDate:   time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			UserID:      "user002",
			Amount:      -25,
			Operation:   models.OperationTransferOut,
			VoucherCode: "sample-voucher-code-123",
			RelatedUser: "user003",
			ExpiryDate:  time.Date(2025, 7, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			UserID:      "user003",
			Amount:      25,
			Operation:   models.OperationTransferIn,
			VoucherCode: "sample-voucher-code-123",
			RelatedUser: "user002",
			ExpiryDate:  time.Date(2025, 7, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			UserID:       "user004",
			Amount:       200,
			Operation:    models.OperationRecharge,
			StrategyName: "new-user-welcome",
			ExpiryDate:   time.Date(2025, 8, 31, 23, 59, 59, 0, time.UTC),
		},
	}

	// Generate more random audit records
	operations := []string{models.OperationRecharge, models.OperationTransferOut, models.OperationTransferIn}
	strategies := []string{"recharge-star-everyday", "vip-daily-bonus", "org-weekly-bonus", "active-user-bonus"}

	for i := 6; i <= 20; i++ {
		userID := fmt.Sprintf("user%03d", i)

		// Generate 1-5 audit records per user
		auditCount := rand.Intn(5) + 1
		for j := 0; j < auditCount; j++ {
			operation := operations[rand.Intn(len(operations))]
			audit := models.QuotaAudit{
				UserID:     userID,
				Amount:     rand.Intn(100) + 10, // 10-110
				Operation:  operation,
				ExpiryDate: time.Date(2025, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 23, 59, 59, 0, time.UTC),
			}

			// Set specific fields based on operation type
			switch operation {
			case models.OperationRecharge:
				audit.StrategyName = strategies[rand.Intn(len(strategies))]
			case models.OperationTransferOut:
				audit.Amount = -audit.Amount // Negative for transfer out
				audit.VoucherCode = fmt.Sprintf("voucher-%d-%d", i, j)
				audit.RelatedUser = fmt.Sprintf("user%03d", rand.Intn(20)+1)
			case models.OperationTransferIn:
				audit.VoucherCode = fmt.Sprintf("voucher-%d-%d", rand.Intn(20)+1, rand.Intn(5)+1)
				audit.RelatedUser = fmt.Sprintf("user%03d", rand.Intn(20)+1)
			}

			audits = append(audits, audit)
		}
	}

	// Batch insert audit data
	for _, audit := range audits {
		if err := db.Create(&audit).Error; err != nil {
			log.Printf("Warning: Failed to create audit for %s: %v", audit.UserID, err)
		}
	}

	fmt.Printf("Generated %d audit records\n", len(audits))
	return nil
}
