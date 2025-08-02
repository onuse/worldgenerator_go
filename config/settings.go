package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Settings struct {
	Simulation SimulationSettings `json:"simulation"`
	Server     ServerSettings     `json:"server"`
	GPU        GPUSettings        `json:"gpu"`
}

type SimulationSettings struct {
	IcosphereLevel int    `json:"icosphereLevel"`
	Comment        string `json:"comment"`
}

type ServerSettings struct {
	Port             int `json:"port"`
	UpdateIntervalMs int `json:"updateIntervalMs"`
}

type GPUSettings struct {
	PreferMetal  bool `json:"preferMetal"`
	PreferOpenCL bool `json:"preferOpenCL"`
}

var globalSettings Settings

func loadSettings() error {
	// Set defaults
	globalSettings = Settings{
		Simulation: SimulationSettings{
			IcosphereLevel: 6,
		},
		Server: ServerSettings{
			Port:             8080,
			UpdateIntervalMs: 100,
		},
		GPU: GPUSettings{
			PreferMetal:  true,
			PreferOpenCL: false,
		},
	}

	// Try to load from file
	file, err := os.Open("settings.json")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No settings.json found, using defaults")
			return nil
		}
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&globalSettings); err != nil {
		return fmt.Errorf("error parsing settings.json: %v", err)
	}

	fmt.Printf("Loaded settings: Icosphere level %d (~%d vertices)\n", 
		globalSettings.Simulation.IcosphereLevel, 
		getApproximateVertexCount(globalSettings.Simulation.IcosphereLevel))
	
	return nil
}

func getApproximateVertexCount(level int) int {
	// Icosphere vertex count formula: 10 * 4^level + 2
	count := 10
	for i := 0; i < level; i++ {
		count *= 4
	}
	return count + 2
}

// Hot reload settings (optional, can be called during runtime)
func reloadSettings() error {
	oldLevel := globalSettings.Simulation.IcosphereLevel
	
	if err := loadSettings(); err != nil {
		return err
	}
	
	if oldLevel != globalSettings.Simulation.IcosphereLevel {
		fmt.Printf("Resolution changed from level %d to %d - restart required\n", 
			oldLevel, globalSettings.Simulation.IcosphereLevel)
	}
	
	return nil
}