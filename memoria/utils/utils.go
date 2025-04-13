package utils

import (
	"os"
	"log"
	"encoding/json"
)

type Config struct {
	PORT_MEMORY int "json:port_memory" 
    MEMORY_SIZE int "json: memory_size"
    PAGE_SIZE int "json: page_size"
    ENTRIES_PER_PAGE int"json: entries_per_page"
    NUMBER_OF_LEVELS int "json: number_of_levels" 
    MEMORY_DELAY int"json: memory_delay"
    SWAPFILE_PATH string "json: swapfile_path"
    SWAP_DELAY string"json: swap_delay"
    LOG_LEVEL  string "json: log_level"
    DUMP_PATH string "json: dump_path"
}

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}
