package service

import (
	"testing"

	"assessv2/backend/internal/model"
)

func TestResolveDependencyConfigsDefaults(t *testing.T) {
	collector := newDependencyIssueCollector(20)
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "Q1"},
		{PeriodCode: "Q2"},
		{PeriodCode: "Q3"},
		{PeriodCode: "Q4"},
		{PeriodCode: "YEAR_END"},
	}
	dependencies := resolveDependencyConfigs(map[string]any{}, periods, collector)
	if len(dependencies) < 2 {
		t.Fatalf("expected at least 2 dependencies, got=%d", len(dependencies))
	}

	hasObjectParent := false
	hasPeriodRollup := false
	for _, dep := range dependencies {
		if dep.Type == dependencyTypeObjectParent {
			hasObjectParent = true
		}
		if dep.Type == dependencyTypePeriodRollup {
			hasPeriodRollup = true
		}
	}
	if !hasObjectParent || !hasPeriodRollup {
		t.Fatalf("expected default dependencies object_parent and period_rollup")
	}
}

func TestResolveDependencyConfigsUsesCustomSessionPeriods(t *testing.T) {
	collector := newDependencyIssueCollector(20)
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "M1"},
		{PeriodCode: "M2"},
		{PeriodCode: "M3"},
	}
	dependencies := resolveDependencyConfigs(map[string]any{}, periods, collector)
	foundRollup := false
	for _, dep := range dependencies {
		if dep.Type != dependencyTypePeriodRollup {
			continue
		}
		foundRollup = true
		if dep.TargetPeriod != "M3" {
			t.Fatalf("unexpected target period, got=%s want=M3", dep.TargetPeriod)
		}
		if len(dep.SourcePeriods) != 2 || dep.SourcePeriods[0] != "M1" || dep.SourcePeriods[1] != "M2" {
			t.Fatalf("unexpected source periods, got=%v want=[M1 M2]", dep.SourcePeriods)
		}
	}
	if !foundRollup {
		t.Fatalf("expected dynamic period_rollup dependency for custom periods")
	}
}

func TestFindDependencyCycles(t *testing.T) {
	graph := newDependencyGraph()
	graph.addEdge("a", "b")
	graph.addEdge("b", "c")
	graph.addEdge("c", "a")

	cycles := findDependencyCycles(graph, 10)
	if len(cycles) == 0 {
		t.Fatalf("expected at least one cycle")
	}
	if len(cycles[0]) < 2 {
		t.Fatalf("unexpected cycle path: %v", cycles[0])
	}
}

func TestCompileDependencyGraphObjectParentCycle(t *testing.T) {
	parentOfTeam := uint(2)
	parentOfIndividual := uint(1)
	objects := []model.AssessmentSessionObject{
		{
			ID:             1,
			ObjectType:     ObjectTypeTeam,
			ParentObjectID: &parentOfTeam,
			IsActive:       true,
		},
		{
			ID:             2,
			ObjectType:     ObjectTypeIndividual,
			ParentObjectID: &parentOfIndividual,
			IsActive:       true,
		},
	}
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "Q1"},
	}
	dependencies := []dependencyConfig{
		{
			Type:             dependencyTypeObjectParent,
			TargetObjectType: ObjectTypeIndividual,
			SourceObjectType: ObjectTypeTeam,
		},
		{
			Type:             dependencyTypeObjectParent,
			TargetObjectType: ObjectTypeTeam,
			SourceObjectType: ObjectTypeIndividual,
		},
	}
	collector := newDependencyIssueCollector(50)
	graph := compileDependencyGraph(periods, objects, dependencies, collector)
	cycles := findDependencyCycles(graph, 10)
	if len(cycles) == 0 {
		t.Fatalf("expected cycle in object parent dependency graph")
	}
}

func TestCompileDependencyGraphMissingParentWarning(t *testing.T) {
	objects := []model.AssessmentSessionObject{
		{
			ID:         100,
			ObjectType: ObjectTypeIndividual,
			IsActive:   true,
		},
	}
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "Q1"},
	}
	dependencies := []dependencyConfig{
		{
			Type:             dependencyTypeObjectParent,
			TargetObjectType: ObjectTypeIndividual,
			SourceObjectType: ObjectTypeTeam,
		},
	}
	collector := newDependencyIssueCollector(50)
	_ = compileDependencyGraph(periods, objects, dependencies, collector)
	issues := collector.all()
	if len(issues) == 0 {
		t.Fatalf("expected warnings for missing parent object")
	}
	found := false
	for _, issue := range issues {
		if issue.Code == dependencyIssueMissingParent {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s issue, got=%v", dependencyIssueMissingParent, issues)
	}
}

func TestCompileDependencyGraphInvalidRollupConfig(t *testing.T) {
	objects := []model.AssessmentSessionObject{
		{
			ID:         1,
			ObjectType: ObjectTypeTeam,
			IsActive:   true,
		},
	}
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "YEAR_END"},
	}
	dependencies := []dependencyConfig{
		{
			Type:          dependencyTypePeriodRollup,
			TargetPeriod:  "YEAR_END",
			SourcePeriods: []string{"YEAR_END"},
		},
	}
	collector := newDependencyIssueCollector(50)
	_ = compileDependencyGraph(periods, objects, dependencies, collector)
	issues := collector.all()
	if len(issues) == 0 {
		t.Fatalf("expected issues for invalid period rollup")
	}
	found := false
	for _, issue := range issues {
		if issue.Code == dependencyIssueInvalidRollup {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s issue, got=%v", dependencyIssueInvalidRollup, issues)
	}
}

func TestCompileDependencyGraphPeriodRollupDerivesTargetFromSessionPeriods(t *testing.T) {
	objects := []model.AssessmentSessionObject{
		{
			ID:         1,
			ObjectType: ObjectTypeTeam,
			IsActive:   true,
		},
	}
	periods := []model.AssessmentSessionPeriod{
		{PeriodCode: "M1"},
		{PeriodCode: "M2"},
		{PeriodCode: "M3"},
	}
	dependencies := []dependencyConfig{
		{
			Type: dependencyTypePeriodRollup,
		},
	}
	collector := newDependencyIssueCollector(50)
	graph := compileDependencyGraph(periods, objects, dependencies, collector)
	if graph.edgeCount() != 2 {
		t.Fatalf("unexpected edge count, got=%d want=2", graph.edgeCount())
	}
	if len(collector.all()) != 0 {
		t.Fatalf("expected no dependency issues, got=%v", collector.all())
	}
}
