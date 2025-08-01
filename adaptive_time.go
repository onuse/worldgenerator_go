package main

import "math"

// AdaptiveTimeStep calculates optimal time step based on current conditions
type AdaptiveTimeStep struct {
	MinStep          float64
	MaxStep          float64
	CurrentStep      float64
	StressThreshold  float64
	LastMaxStress    float64
}

// NewAdaptiveTimeStep creates a new adaptive time stepper
func NewAdaptiveTimeStep() *AdaptiveTimeStep {
	return &AdaptiveTimeStep{
		MinStep:         1000.0,      // 1,000 years minimum
		MaxStep:         10000000.0,  // 10 million years maximum
		CurrentStep:     1000.0,      // Start conservative
		StressThreshold: 0.01,        // Stress level that triggers smaller steps
		LastMaxStress:   0.0,
	}
}

// CalculateNextStep determines the next time step based on plate stress
func (ats *AdaptiveTimeStep) CalculateNextStep(planet Planet, requestedSpeed float64) float64 {
	// Calculate maximum stress at boundaries
	maxStress := ats.calculateMaxStress(planet)
	ats.LastMaxStress = maxStress
	
	// Base step from requested speed (years per frame at 10 fps)
	baseStep := requestedSpeed / 10.0
	
	// Adjust based on stress
	if maxStress > ats.StressThreshold {
		// High stress - use smaller steps
		ats.CurrentStep = math.Max(ats.MinStep, baseStep * 0.1)
	} else if maxStress < ats.StressThreshold * 0.5 {
		// Low stress - can use larger steps
		ats.CurrentStep = math.Min(ats.MaxStep, baseStep * 2.0)
	} else {
		// Medium stress - normal steps
		ats.CurrentStep = baseStep
	}
	
	// Clamp to limits
	ats.CurrentStep = math.Max(ats.MinStep, math.Min(ats.MaxStep, ats.CurrentStep))
	
	return ats.CurrentStep
}

// calculateMaxStress finds the maximum stress at plate boundaries
func (ats *AdaptiveTimeStep) calculateMaxStress(planet Planet) float64 {
	maxStress := 0.0
	
	// Check stress at boundaries
	for _, boundary := range planet.Boundaries {
		if boundary.Plate1 >= len(planet.Plates) || boundary.Plate2 >= len(planet.Plates) {
			continue
		}
		
		plate1 := planet.Plates[boundary.Plate1]
		plate2 := planet.Plates[boundary.Plate2]
		
		// Calculate relative velocity
		relVel := plate1.Velocity.Add(plate2.Velocity.Scale(-1))
		stress := relVel.Length()
		
		// Extra stress for continental collision
		if plate1.Type == Continental && plate2.Type == Continental {
			stress *= 2.0
		}
		
		if stress > maxStress {
			maxStress = stress
		}
	}
	
	return maxStress
}

// ShouldUpdateBoundaries decides if boundaries need recalculation
func (ats *AdaptiveTimeStep) ShouldUpdateBoundaries(planet Planet, lastBoundaryUpdate float64) bool {
	timeSinceUpdate := planet.GeologicalTime - lastBoundaryUpdate
	
	// Update more frequently during high stress
	if ats.LastMaxStress > ats.StressThreshold {
		return timeSinceUpdate > 50000 // 50k years
	}
	
	// Normal update frequency
	return timeSinceUpdate > 200000 // 200k years
}

// GetStepInfo returns information about the current time stepping
func (ats *AdaptiveTimeStep) GetStepInfo() string {
	quality := "Normal"
	if ats.CurrentStep <= ats.MinStep {
		quality = "High Detail"
	} else if ats.CurrentStep >= ats.MaxStep * 0.5 {
		quality = "Fast Forward"
	}
	
	return quality
}