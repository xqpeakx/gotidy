package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdaptiveLearningRemembersExtensionDestination(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "sales.csv", "forecast.csv")

	resolver, err := NewCategoryResolver(&Config{
		Categories: map[string]ConfiguredCategory{
			"data_files": {
				Extensions:  []string{"csv"},
				Destination: "Work/Data",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewCategoryResolver: %v", err)
	}

	categorizer, err := NewCategorizer(dir, resolver, true, true, false)
	if err != nil {
		t.Fatalf("NewCategorizer: %v", err)
	}

	if _, err := Organize(dir, Options{
		Resolver:    resolver,
		Categorizer: categorizer,
	}); err != nil {
		t.Fatalf("Organize with learning: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, learningManifestName)); err != nil {
		t.Fatalf("learning manifest should exist: %v", err)
	}

	writeFiles(t, dir, "report.csv")

	defaultResolver := DefaultCategoryResolver()
	learnedCategorizer, err := NewCategorizer(dir, defaultResolver, true, false, false)
	if err != nil {
		t.Fatalf("NewCategorizer: %v", err)
	}

	summary, err := Organize(dir, Options{
		Resolver:    defaultResolver,
		Categorizer: learnedCategorizer,
	})
	if err != nil {
		t.Fatalf("Organize with adaptive learning: %v", err)
	}
	if summary.AdaptiveMatches != 1 {
		t.Fatalf("AdaptiveMatches = %d, want 1", summary.AdaptiveMatches)
	}
	if _, err := os.Stat(filepath.Join(dir, "Work", "Data", "report.csv")); err != nil {
		t.Fatalf("report.csv should move to learned destination: %v", err)
	}
}

func TestAdaptiveHeuristicUsesSiblingStem(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Work", "Data"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFiles(t, filepath.Join(dir, "Work", "Data"), "budget.csv")

	categorizer, err := NewCategorizer(dir, DefaultCategoryResolver(), true, false, false)
	if err != nil {
		t.Fatalf("NewCategorizer: %v", err)
	}

	decision, err := categorizer.Classify("budget.txt")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if decision.Source != "heuristic-stem" {
		t.Fatalf("decision.Source = %q, want %q", decision.Source, "heuristic-stem")
	}
	if decision.Rule.Name != "spreadsheets" {
		t.Fatalf("decision.Rule.Name = %q, want %q", decision.Rule.Name, "spreadsheets")
	}
	if decision.Rule.Destination != "Work/Data" {
		t.Fatalf("decision.Rule.Destination = %q, want %q", decision.Rule.Destination, "Work/Data")
	}
}

func TestAdaptiveContentHintsPromoteBudgetTextToSpreadsheets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.txt")
	data := "month,amount,balance\nJanuary,100,1000\nFebruary,120,1120\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	categorizer, err := NewCategorizer(dir, DefaultCategoryResolver(), true, false, true)
	if err != nil {
		t.Fatalf("NewCategorizer: %v", err)
	}

	decision, err := categorizer.Classify(path)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if decision.Source != "content-hint" {
		t.Fatalf("decision.Source = %q, want %q", decision.Source, "content-hint")
	}
	if decision.Rule.Name != "spreadsheets" {
		t.Fatalf("decision.Rule.Name = %q, want %q", decision.Rule.Name, "spreadsheets")
	}
}
