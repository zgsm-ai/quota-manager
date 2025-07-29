package main

import (
	"fmt"
	"quota-manager/internal/config"
)

// testGithubStarCheckDisabled tests behavior when github_star_check is disabled
func testGithubStarCheckDisabled(ctx *TestContext) TestResult {
	// Save original config value
	originalEnabled := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.Enabled

	// Temporarily disable the feature
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = false
	})

	// Create a test user
	user := createTestUser("github_star_disabled_user", "GitHub Star Disabled User", 0)
	// Set GithubStar field to contain the required repo
	user.GithubStar = "zgsm-ai.zgsm,openai.gpt-4"
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Call GetUserQuota method
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("GetUserQuota failed: %v", err)}
	}

	// Verify that IsStar field is not present (empty string)
	if quotaInfo.IsStar != "" {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
		})
		return TestResult{Passed: false, Message: "IsStar field should be empty when feature is disabled"}
	}

	// Restore original config
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = originalEnabled
	})

	return TestResult{Passed: true, Message: "Test Config Disabled Succeeded"}
}

// testGithubStarCheckEnabledUserNotStar tests behavior when user hasn't starred the repository
func testGithubStarCheckEnabledUserNotStar(ctx *TestContext) TestResult {
	// Save original config values
	originalEnabled := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.Enabled
	originalRepo := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.RequiredRepo

	// Enable the feature and set repository
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = true
		cfg.GithubStarCheck.RequiredRepo = "zgsm-ai.zgsm"
	})

	// Create a test user who hasn't starred the repository
	user := createTestUser("github_not_starred_user", "GitHub Not Starred User", 0)
	// User who hasn't starred the required repo
	user.GithubStar = "openai.gpt-4,other.repo"
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Initialize mock quota for the user
	mockStore.SetQuota(user.ID, 100)

	// Call GetUserQuota method
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("GetUserQuota failed: %v", err)}
	}

	// Verify that IsStar field is "false"
	if quotaInfo.IsStar != "false" {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("IsStar field should be 'false', got: %s", quotaInfo.IsStar)}
	}

	// Restore original config
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = originalEnabled
		cfg.GithubStarCheck.RequiredRepo = originalRepo
	})

	return TestResult{Passed: true, Message: "Test User Not Starred Succeeded"}
}

// testGithubStarCheckEnabledUserStarred tests behavior when user has starred the repository
func testGithubStarCheckEnabledUserStarred(ctx *TestContext) TestResult {
	// Save original config values
	originalEnabled := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.Enabled
	originalRepo := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.RequiredRepo

	// Enable the feature and set repository
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = true
		cfg.GithubStarCheck.RequiredRepo = "zgsm-ai.zgsm"
	})

	// Create a test user who has starred the repository
	user := createTestUser("github_has_starred_user", "GitHub Has Starred User", 0)
	// User who has starred the required repo
	user.GithubStar = "zgsm-ai.zgsm,openai.gpt-4"
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Initialize mock quota for the user
	mockStore.SetQuota(user.ID, 100)

	// Call GetUserQuota method
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("GetUserQuota failed: %v", err)}
	}

	// Verify that IsStar field is "true"
	if quotaInfo.IsStar != "true" {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("IsStar field should be 'true', got: %s", quotaInfo.IsStar)}
	}

	// Restore original config
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = originalEnabled
		cfg.GithubStarCheck.RequiredRepo = originalRepo
	})

	return TestResult{Passed: true, Message: "Test User Has Starred Succeeded"}
}

// testGithubStarCheckEnabledUserStarredOther tests behavior when user starred a different repository
func testGithubStarCheckEnabledUserStarredOther(ctx *TestContext) TestResult {
	// Save original config values
	originalEnabled := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.Enabled
	originalRepo := ctx.QuotaService.GetConfigManager().GetDirect().GithubStarCheck.RequiredRepo

	// Enable the feature and set repository
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = true
		cfg.GithubStarCheck.RequiredRepo = "zgsm-ai.zgsm"
	})

	// Create a test user who has starred a different repository
	user := createTestUser("github_different_repo_user", "GitHub Different Repo User", 0)
	// User who starred a different repo
	user.GithubStar = "openai.gpt-4,other.repo"
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Initialize mock quota for the user
	mockStore.SetQuota(user.ID, 100)

	// Call GetUserQuota method
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("GetUserQuota failed: %v", err)}
	}

	// Verify that IsStar field is "false"
	if quotaInfo.IsStar != "false" {
		// Restore config
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.GithubStarCheck.Enabled = originalEnabled
			cfg.GithubStarCheck.RequiredRepo = originalRepo
		})
		return TestResult{Passed: false, Message: fmt.Sprintf("IsStar field should be 'false', got: %s", quotaInfo.IsStar)}
	}

	// Restore original config
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.GithubStarCheck.Enabled = originalEnabled
		cfg.GithubStarCheck.RequiredRepo = originalRepo
	})

	return TestResult{Passed: true, Message: "Test User Starred Different Repo Succeeded"}
}
