package service

import (
	"errors"
	"testing"
)

func TestExpressionRuntimeCompileAndEval(t *testing.T) {
	runtime := NewExpressionRuntime()

	programA, err := runtime.CompileNumber("moduleA * 0.7 + moduleB * 0.3")
	if err != nil {
		t.Fatalf("compile number failed: %v", err)
	}
	programB, err := runtime.CompileNumber("moduleA * 0.7 + moduleB * 0.3")
	if err != nil {
		t.Fatalf("compile number from cache failed: %v", err)
	}
	if programA != programB {
		t.Fatalf("expected number program cache hit")
	}

	score, err := runtime.EvalNumber("moduleA * 0.7 + moduleB * 0.3", map[string]any{
		"moduleA": 80,
		"moduleB": 90,
	})
	if err != nil {
		t.Fatalf("eval number failed: %v", err)
	}
	if !almostEqual(score, 83) {
		t.Fatalf("unexpected score, got=%v want=83", score)
	}

	boolProgramA, err := runtime.CompileBool("totalScore >= 90")
	if err != nil {
		t.Fatalf("compile bool failed: %v", err)
	}
	boolProgramB, err := runtime.CompileBool("totalScore >= 90")
	if err != nil {
		t.Fatalf("compile bool from cache failed: %v", err)
	}
	if boolProgramA != boolProgramB {
		t.Fatalf("expected bool program cache hit")
	}

	passed, err := runtime.EvalBool("totalScore >= 90", map[string]any{
		"totalScore": 92.5,
	})
	if err != nil {
		t.Fatalf("eval bool failed: %v", err)
	}
	if !passed {
		t.Fatalf("expected bool result true")
	}
}

func TestExpressionRuntimeCompileErrorMapping(t *testing.T) {
	runtime := NewExpressionRuntime()
	if _, err := runtime.CompileNumber("1 +"); !errors.Is(err, ErrInvalidExpression) {
		t.Fatalf("expected ErrInvalidExpression, got=%v", err)
	}
	if _, err := runtime.CompileBool("   "); !errors.Is(err, ErrInvalidExpression) {
		t.Fatalf("expected ErrInvalidExpression for empty bool expression, got=%v", err)
	}
}

func TestExpressionRuntimeEvalErrorMapping(t *testing.T) {
	runtime := NewExpressionRuntime()
	if _, err := runtime.EvalNumber("1 +", map[string]any{}); !errors.Is(err, ErrCalcExpressionEval) {
		t.Fatalf("expected ErrCalcExpressionEval for compile failure, got=%v", err)
	}
	if _, err := runtime.EvalNumber("unknown + 1", map[string]any{}); !errors.Is(err, ErrCalcExpressionEval) {
		t.Fatalf("expected ErrCalcExpressionEval for runtime failure, got=%v", err)
	}
	if _, err := runtime.EvalBool("score + 1", map[string]any{"score": 1}); !errors.Is(err, ErrCalcExpressionEval) {
		t.Fatalf("expected ErrCalcExpressionEval for bool type mismatch, got=%v", err)
	}
}

func TestExpressionRuntimeCompileSupportsLookupFunctions(t *testing.T) {
	runtime := NewExpressionRuntime()
	if _, err := runtime.CompileNumber(`score("Q1", objectId) + moduleScore("Q1", objectId, "base_performance")`); err != nil {
		t.Fatalf("compile with lookup functions failed: %v", err)
	}
	if _, err := runtime.CompileNumber(`rank("Q1", objectId) + score("Q1", objectId)`); err != nil {
		t.Fatalf("compile with rank lookup failed: %v", err)
	}
	if _, err := runtime.CompileBool(`hasScore("Q1", objectId) && targetScore("Q1", "department", targetId) >= 80`); err != nil {
		t.Fatalf("compile bool with lookup functions failed: %v", err)
	}
	if _, err := runtime.CompileBool(`grade("Q1", objectId) == "A"`); err != nil {
		t.Fatalf("compile bool with grade lookup failed: %v", err)
	}
}
