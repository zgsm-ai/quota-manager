package condition

import (
	"fmt"
	"quota-manager/internal/models"
	"strconv"
	"strings"
	"time"
)

// QuotaQuerier interface for querying quota information
type QuotaQuerier interface {
	QueryQuota(userID string) (int, error)
}

// DatabaseQuerier interface for querying database information
type DatabaseQuerier interface {
	QueryEmployeeDepartment(employeeNumber string) ([]string, error)
}

// ConfigQuerier interface for accessing configuration
type ConfigQuerier interface {
	IsEmployeeSyncEnabled() bool
}

// EvaluationContext contains all dependencies needed for condition evaluation
type EvaluationContext struct {
	QuotaQuerier    QuotaQuerier
	DatabaseQuerier DatabaseQuerier
	ConfigQuerier   ConfigQuerier
	// Can add more dependencies here in the future (e.g., cache, etc.)
}

type Parser struct {
	tokens []string
	pos    int
}

type Evaluator interface {
	Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error)
}

// AndExpr logical AND expression
type AndExpr struct {
	Left, Right Evaluator
}

func (a *AndExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	left, err := a.Left.Evaluate(user, ctx)
	if err != nil || !left {
		return false, err
	}
	return a.Right.Evaluate(user, ctx)
}

// OrExpr logical OR expression
type OrExpr struct {
	Left, Right Evaluator
}

func (o *OrExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	left, err := o.Left.Evaluate(user, ctx)
	if err != nil {
		return false, err
	}
	if left {
		return true, nil
	}
	return o.Right.Evaluate(user, ctx)
}

// NotExpr logical NOT expression
type NotExpr struct {
	Expr Evaluator
}

func (n *NotExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	result, err := n.Expr.Evaluate(user, ctx)
	return !result, err
}

// MatchUserExpr match user expression
type MatchUserExpr struct {
	UserID string
}

func (m *MatchUserExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return user.ID == m.UserID, nil
}

// RegisterBeforeExpr registration time before expression
type RegisterBeforeExpr struct {
	Timestamp time.Time
}

func (r *RegisterBeforeExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return !user.CreatedAt.After(r.Timestamp), nil
}

// AccessAfterExpr access time after expression
type AccessAfterExpr struct {
	Timestamp time.Time
}

func (a *AccessAfterExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return user.AccessTime.After(a.Timestamp), nil
}

// GithubStarExpr GitHub star expression
type GithubStarExpr struct {
	Project string
}

func (g *GithubStarExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	if user.GithubStar == "" {
		return false, nil
	}
	stars := strings.Split(user.GithubStar, ",")
	for _, star := range stars {
		if strings.TrimSpace(star) == g.Project {
			return true, nil
		}
	}
	return false, nil
}

// QuotaLEExpr quota less than or equal expression
type QuotaLEExpr struct {
	Model  string
	Amount int
}

func (q *QuotaLEExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	if ctx.QuotaQuerier == nil {
		return false, fmt.Errorf("quota querier not available")
	}

	quota, err := ctx.QuotaQuerier.QueryQuota(user.ID)
	if err != nil {
		return false, err
	}
	return quota <= q.Amount, nil
}

// IsVipExpr VIP level expression
type IsVipExpr struct {
	Level int
}

func (i *IsVipExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return user.VIP >= i.Level, nil
}

// BelongToExpr belongs to organization expression
type BelongToExpr struct {
	Org string
}

func (b *BelongToExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	// Check if employee sync is enabled and we have the necessary dependencies
	if ctx.ConfigQuerier != nil && ctx.ConfigQuerier.IsEmployeeSyncEnabled() &&
		ctx.DatabaseQuerier != nil && user.EmployeeNumber != "" {

		// Use new logic: check if employee belongs to department via employee_department table
		departments, err := ctx.DatabaseQuerier.QueryEmployeeDepartment(user.EmployeeNumber)
		if err != nil {
			// If query fails, fall back to original logic
			return user.Company == b.Org, nil
		}

		// Check if the specified department/organization exists in user's department hierarchy
		// Support both Chinese and English department names
		for _, dept := range departments {
			if dept == b.Org {
				return true, nil
			}
		}

		return false, nil
	}

	// Fall back to original logic when employee sync is disabled or dependencies are missing
	return user.Company == b.Org, nil
}

// TrueExpr always returns true
type TrueExpr struct{}

func (t *TrueExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return true, nil
}

// FalseExpr always returns false
type FalseExpr struct{}

func (f *FalseExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	return false, nil
}

// RechargeExpr already recharged expression
type RechargeExpr struct {
	StrategyName string
	DB           interface {
		Where(query interface{}, args ...interface{}) interface {
			First(dest interface{}) interface{ Error() error }
		}
	}
}

func (r *RechargeExpr) Evaluate(user *models.UserInfo, ctx *EvaluationContext) (bool, error) {
	var execute models.QuotaExecute
	err := r.DB.Where("user = ? AND status = 'completed'", user.ID).First(&execute).Error()
	return err == nil, nil
}

func NewParser(condition string) *Parser {
	if condition == "" {
		return &Parser{tokens: []string{}, pos: 0}
	}
	tokens := tokenize(condition)
	return &Parser{tokens: tokens, pos: 0}
}

func tokenize(condition string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	inParens := 0

	for _, r := range condition {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case '(':
			if !inQuotes {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, "(")
				inParens++
			} else {
				current.WriteRune(r)
			}
		case ')':
			if !inQuotes {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, ")")
				inParens--
			} else {
				current.WriteRune(r)
			}
		case ',', ' ':
			if !inQuotes && inParens == 0 {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else if !inQuotes && inParens > 0 && r == ',' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, ",")
			} else if r != ' ' || inQuotes {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (p *Parser) Parse() (Evaluator, error) {
	if len(p.tokens) == 0 {
		return nil, fmt.Errorf("empty condition is not allowed, use true() for always-true condition")
	}
	return p.parseOr()
}

func (p *Parser) parseOr() (Evaluator, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.pos < len(p.tokens) && p.tokens[p.pos] == "or" {
		p.pos++ // consume 'or'
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &OrExpr{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseAnd() (Evaluator, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.pos < len(p.tokens) && p.tokens[p.pos] == "and" {
		p.pos++ // consume 'and'
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &AndExpr{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseUnary() (Evaluator, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	token := p.tokens[p.pos]

	if token == "not" {
		p.pos++ // consume 'not'
		if p.pos >= len(p.tokens) || p.tokens[p.pos] != "(" {
			return nil, fmt.Errorf("expected '(' after 'not'")
		}
		expr, err := p.parseFunction()
		if err != nil {
			return nil, err
		}
		return &NotExpr{Expr: expr}, nil
	}

	return p.parseFunction()
}

func (p *Parser) parseFunction() (Evaluator, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	if p.tokens[p.pos] == "(" {
		p.pos++ // consume '('
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos] != ")" {
			return nil, fmt.Errorf("expected ')' but got %s", p.currentToken())
		}
		p.pos++ // consume ')'
		return expr, nil
	}

	funcName := p.tokens[p.pos]
	p.pos++ // consume function name

	if p.pos >= len(p.tokens) || p.tokens[p.pos] != "(" {
		return nil, fmt.Errorf("expected '(' after function name")
	}
	p.pos++ // consume '('

	// Special handling for logical operator functions
	if funcName == "and" || funcName == "or" || funcName == "not" {
		var args []Evaluator
		for p.pos < len(p.tokens) && p.tokens[p.pos] != ")" {
			if p.tokens[p.pos] == "," {
				p.pos++
				continue
			}
			arg, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		if p.pos >= len(p.tokens) {
			return nil, fmt.Errorf("expected ')' to close function")
		}
		p.pos++ // consume ')'

		switch funcName {
		case "and":
			if len(args) != 2 {
				return nil, fmt.Errorf("and function expects 2 arguments, got %d", len(args))
			}
			return &AndExpr{Left: args[0], Right: args[1]}, nil
		case "or":
			if len(args) != 2 {
				return nil, fmt.Errorf("or function expects 2 arguments, got %d", len(args))
			}
			return &OrExpr{Left: args[0], Right: args[1]}, nil
		case "not":
			if len(args) != 1 {
				return nil, fmt.Errorf("not function expects 1 argument, got %d", len(args))
			}
			return &NotExpr{Expr: args[0]}, nil
		}
	}

	// Handle regular functions
	var args []string
	for p.pos < len(p.tokens) && p.tokens[p.pos] != ")" {
		if p.tokens[p.pos] == "," {
			p.pos++
			continue
		}
		args = append(args, p.tokens[p.pos])
		p.pos++
	}

	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("expected ')' to close function")
	}
	p.pos++ // consume ')'

	return p.buildFunction(funcName, args)
}

func (p *Parser) buildFunction(funcName string, args []string) (Evaluator, error) {
	switch funcName {
	case "match-user":
		if len(args) != 1 {
			return nil, fmt.Errorf("match-user expects 1 argument, got %d", len(args))
		}
		return &MatchUserExpr{UserID: strings.Trim(args[0], "\"")}, nil

	case "register-before":
		if len(args) != 1 {
			return nil, fmt.Errorf("register-before expects 1 argument, got %d", len(args))
		}
		timestamp, err := time.Parse("2006-01-02 15:04:05", strings.Trim(args[0], "\""))
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp format: %w", err)
		}
		return &RegisterBeforeExpr{Timestamp: timestamp}, nil

	case "access-after":
		if len(args) != 1 {
			return nil, fmt.Errorf("access-after expects 1 argument, got %d", len(args))
		}
		timestamp, err := time.Parse("2006-01-02 15:04:05", strings.Trim(args[0], "\""))
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp format: %w", err)
		}
		return &AccessAfterExpr{Timestamp: timestamp}, nil

	case "github-star":
		if len(args) != 1 {
			return nil, fmt.Errorf("github-star expects 1 argument, got %d", len(args))
		}
		return &GithubStarExpr{Project: strings.Trim(args[0], "\"")}, nil

	case "quota-le":
		if len(args) != 2 {
			return nil, fmt.Errorf("quota-le expects 2 arguments, got %d", len(args))
		}
		amount, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid amount: %w", err)
		}
		return &QuotaLEExpr{Model: strings.Trim(args[0], "\""), Amount: amount}, nil

	case "is-vip":
		if len(args) != 1 {
			return nil, fmt.Errorf("is-vip expects 1 argument, got %d", len(args))
		}
		level, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("invalid vip level: %w", err)
		}
		return &IsVipExpr{Level: level}, nil

	case "belong-to":
		if len(args) != 1 {
			return nil, fmt.Errorf("belong-to expects 1 argument, got %d", len(args))
		}
		return &BelongToExpr{Org: strings.Trim(args[0], "\"")}, nil

	case "true":
		if len(args) != 0 {
			return nil, fmt.Errorf("true expects 0 arguments, got %d", len(args))
		}
		return &TrueExpr{}, nil

	case "false":
		if len(args) != 0 {
			return nil, fmt.Errorf("false expects 0 arguments, got %d", len(args))
		}
		return &FalseExpr{}, nil

	default:
		return nil, fmt.Errorf("unknown function: %s", funcName)
	}
}

func (p *Parser) currentToken() string {
	if p.pos >= len(p.tokens) {
		return "EOF"
	}
	return p.tokens[p.pos]
}

// CalcCondition calculate condition expression
func CalcCondition(user *models.UserInfo, condition string, ctx *EvaluationContext) (bool, error) {
	if condition == "" {
		return false, fmt.Errorf("empty condition is not allowed, use true() for always-true condition")
	}

	parser := NewParser(condition)
	evaluator, err := parser.Parse()
	if err != nil {
		return false, fmt.Errorf("failed to parse condition: %w", err)
	}

	return evaluator.Evaluate(user, ctx)
}
