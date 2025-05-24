package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("../config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 连接数据库
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	// 生成用户数据
	if err := generateUsers(db); err != nil {
		log.Fatalf("Failed to generate users: %v", err)
	}

	// 生成策略数据
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
			Name:           "张三",
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
			Name:           "李四",
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
			Name:           "王五",
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
			Name:           "赵六",
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
			Name:           "孙七",
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

	// 生成更多随机用户
	for i := 6; i <= 20; i++ {
		user := models.UserInfo{
			ID:             fmt.Sprintf("user%03d", i),
			Name:           fmt.Sprintf("用户%d", i),
			GithubUsername: fmt.Sprintf("user%d", i),
			Email:          fmt.Sprintf("user%d@example.com", i),
			Phone:          fmt.Sprintf("138%08d", rand.Intn(100000000)),
			VIP:            rand.Intn(4),
			RegisterTime:   time.Date(2023, time.Month(rand.Intn(12)+1), rand.Intn(28)+1, 0, 0, 0, 0, time.UTC),
			AccessTime:     time.Now().Add(-time.Duration(rand.Intn(168)) * time.Hour),
		}

		// 随机分配组织
		if rand.Float32() < 0.7 {
			user.Org = fmt.Sprintf("org%03d", rand.Intn(5)+1)
		}

		// 随机分配GitHub star
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

	// 批量插入用户数据
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
			Title:        "给点star用户每日充值",
			Type:         "periodic",
			Amount:       5,
			Model:        "claude-3-5-sonnet-latest",
			PeriodicExpr: "0 8 * * *", // 每天8点
			Condition:    `github-star("zgsm")`,
		},
		{
			Name:      "recharge-star-once",
			Title:     "给star用户一次性充值",
			Type:      "single",
			Amount:    20,
			Model:     "claude-3-5-sonnet-latest",
			Condition: `github-star("zgsm")`,
		},
		{
			Name:         "vip-daily-bonus",
			Title:        "VIP用户每日奖励",
			Type:         "periodic",
			Amount:       10,
			Model:        "gpt-4",
			PeriodicExpr: "0 9 * * *", // 每天9点
			Condition:    `is-vip(1)`,
		},
		{
			Name:      "new-user-welcome",
			Title:     "新用户欢迎奖励",
			Type:      "single",
			Amount:    50,
			Model:     "gpt-3.5-turbo",
			Condition: `register-before("2024-06-01 00:00:00")`,
		},
		{
			Name:         "org-weekly-bonus",
			Title:        "组织用户周奖励",
			Type:         "periodic",
			Amount:       15,
			Model:        "claude-3-5-sonnet-latest",
			PeriodicExpr: "0 10 * * 1", // 每周一10点
			Condition:    `belong-to("org001")`,
		},
		{
			Name:      "active-user-bonus",
			Title:     "活跃用户奖励",
			Type:      "single",
			Amount:    30,
			Model:     "gpt-4",
			Condition: `and(access-after("2024-05-01 00:00:00"), is-vip(2))`,
		},
		{
			Name:         "low-quota-refill",
			Title:        "低配额自动补充",
			Type:         "periodic",
			Amount:       25,
			Model:        "gpt-3.5-turbo",
			PeriodicExpr: "0 */6 * * *", // 每6小时
			Condition:    `quota-le("gpt-3.5-turbo", 10)`,
		},
	}

	// 批量插入策略数据
	for _, strategy := range strategies {
		if err := db.Create(&strategy).Error; err != nil {
			fmt.Printf("Warning: Failed to create strategy %s: %v\n", strategy.Name, err)
		}
	}

	fmt.Printf("Generated %d strategies\n", len(strategies))
	return nil
}