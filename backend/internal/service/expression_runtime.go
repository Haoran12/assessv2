package service

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type ExpressionRuntime struct {
	numberProgramCache sync.Map
	boolProgramCache   sync.Map
}

var defaultExpressionRuntime = NewExpressionRuntime()

func NewExpressionRuntime() *ExpressionRuntime {
	return &ExpressionRuntime{}
}

func CompileNumber(script string) (*vm.Program, error) {
	return defaultExpressionRuntime.CompileNumber(script)
}

func CompileBool(script string) (*vm.Program, error) {
	return defaultExpressionRuntime.CompileBool(script)
}

func EvalNumber(script string, env any) (float64, error) {
	return defaultExpressionRuntime.EvalNumber(script, env)
}

func EvalBool(script string, env any) (bool, error) {
	return defaultExpressionRuntime.EvalBool(script, env)
}

func (r *ExpressionRuntime) CompileNumber(script string) (*vm.Program, error) {
	cacheKey, err := normalizeExpression(script)
	if err != nil {
		return nil, err
	}
	if cached, ok := r.numberProgramCache.Load(cacheKey); ok {
		program, _ := cached.(*vm.Program)
		return program, nil
	}
	program, err := expr.Compile(
		cacheKey,
		expr.Env(map[string]any{}),
		expr.AllowUndefinedVariables(),
		expr.AsFloat64(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidExpression, err)
	}
	r.numberProgramCache.Store(cacheKey, program)
	return program, nil
}

func (r *ExpressionRuntime) CompileBool(script string) (*vm.Program, error) {
	cacheKey, err := normalizeExpression(script)
	if err != nil {
		return nil, err
	}
	if cached, ok := r.boolProgramCache.Load(cacheKey); ok {
		program, _ := cached.(*vm.Program)
		return program, nil
	}
	program, err := expr.Compile(
		cacheKey,
		expr.Env(map[string]any{}),
		expr.AllowUndefinedVariables(),
		expr.AsBool(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidExpression, err)
	}
	r.boolProgramCache.Store(cacheKey, program)
	return program, nil
}

func (r *ExpressionRuntime) EvalNumber(script string, env any) (float64, error) {
	program, err := r.CompileNumber(script)
	if err != nil {
		return 0, mapToEvalError(err)
	}
	value, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrCalcExpressionEval, err)
	}
	number, ok := toFloat64(value)
	if !ok {
		return 0, fmt.Errorf("%w: expression result is not number", ErrCalcExpressionEval)
	}
	return number, nil
}

func (r *ExpressionRuntime) EvalBool(script string, env any) (bool, error) {
	program, err := r.CompileBool(script)
	if err != nil {
		return false, mapToEvalError(err)
	}
	value, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrCalcExpressionEval, err)
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%w: expression result is not bool", ErrCalcExpressionEval)
	}
	return result, nil
}

func normalizeExpression(script string) (string, error) {
	text := strings.TrimSpace(script)
	if text == "" {
		return "", fmt.Errorf("%w: expression is empty", ErrInvalidExpression)
	}
	return text, nil
}

func mapToEvalError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrCalcExpressionEval, err)
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
