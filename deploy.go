package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type ConfigMemoria struct {
	PORT_MEMORY      int    `json:"port_memory"`
	IP_MEMORY        string `json:"ip_memory"`
	MEMORY_SIZE      int    `json:"memory_size"`
	PAGE_SIZE        int    `json:"page_size"`
	ENTRIES_PER_PAGE int    `json:"entries_per_page"`
	NUMBER_OF_LEVELS int    `json:"number_of_levels"`
	MEMORY_DELAY     int    `json:"memory_delay"`
	SWAPFILE_PATH    string `json:"swapfile_path"`
	SWAP_DELAY       int    `json:"swap_delay"`
	LOG_LEVEL        string `json:"log_level"`
	DUMP_PATH        string `json:"dump_path"`
	SCRIPTS_PATH     string `json:"scripts_path"`
}

type ConfigKernel struct {
	IP_MEMORY               string  `json:"ip_memory"`
	PORT_MEMORY             int     `json:"port_memory"`
	IP_KERNEL               string  `json:"ip_kernel"`
	PORT_KERNEL             int     `json:"port_kernel"`
	SCHEDULER_ALGORITHM     string  `json:"scheduler_algorithm"`
	READY_INGRESS_ALGORITHM string  `json:"ready_ingress_algorithm"`
	ALPHA                   float32 `json:"alpha"`
	INITIAL_ESTIMATE        float32 `json:"initial_estimate"`
	SUSPENSION_TIME         int     `json:"suspension_time"`
	LOG_LEVEL               string  `json:"log_level"`
}

type ConfigCPU struct {
	PORT_CPU          int    `json:"port_cpu"`
	IP_CPU            string `json:"ip_cpu"`
	PORT_MEMORY       int    `json:"port_memory"`
	IP_MEMORY         string `json:"ip_memory"`
	PORT_KERNEL       int    `json:"port_kernel"`
	IP_KERNEL         string `json:"ip_kernel"`
	TLB_ENTRIES       int    `json:"tlb_entries"`
	TLB_REPLACEMENT   string `json:"tlb_replacement"`
	CACHE_ENTRIES     int    `json:"cache_entries"`
	CACHE_REPLACEMENT string `json:"cache_replacement"`
	CACHE_DELAY       int    `json:"cache_delay"`
	LOG_LEVEL         string `json:"log_level"`
}

type ConfigIO struct {
	PORT_IO     int    `json:"port_io"`
	IP_IO       string `json:"ip_io"`
	IP_KERNEL   string `json:"ip_kernel"`
	PORT_KERNEL int    `json:"port_kernel"`
	LOG_LEVEL   string `json:"log_level"`
}

var rutaArchivo string

var configuracionCPU = ConfigCPU{
	PORT_CPU:          8004,
	IP_CPU:            "127.0.0.1",
	PORT_MEMORY:       8002,
	IP_MEMORY:         "127.0.0.1",
	PORT_KERNEL:       8001,
	IP_KERNEL:         "127.0.0.1",
	TLB_ENTRIES:       4,
	TLB_REPLACEMENT:   "LRU",
	CACHE_ENTRIES:     2,
	CACHE_REPLACEMENT: "CLOCK",
	CACHE_DELAY:       250,
	LOG_LEVEL:         "INFO",
}

var configuracionMemoria = ConfigMemoria{
	PORT_MEMORY:      8002,
	IP_MEMORY:        "127.0.0.1",
	MEMORY_SIZE:      4096,
	PAGE_SIZE:        64,
	ENTRIES_PER_PAGE: 4,
	NUMBER_OF_LEVELS: 2,
	MEMORY_DELAY:     500,
	SWAPFILE_PATH:    "/swapfile.bin",
	SWAP_DELAY:       15000,
	LOG_LEVEL:        "INFO",
	DUMP_PATH:        "",
	SCRIPTS_PATH:     "/home/utnso/ssoo/golang/tp-2025-1c-Harkcoded/globales/archivos_prueba",
}

var configuracionKernel = ConfigKernel{
	IP_MEMORY:               "127.0.0.1",
	PORT_MEMORY:             8002,
	IP_KERNEL:               "127.0.0.1",
	PORT_KERNEL:             8001,
	SCHEDULER_ALGORITHM:     "FIFO",
	READY_INGRESS_ALGORITHM: "FIFO",
	ALPHA:                   1,
	INITIAL_ESTIMATE:        1000,
	SUSPENSION_TIME:         12000,
	LOG_LEVEL:               "INFO",
}

var configIO = ConfigIO{
	PORT_IO:     8003,
	IP_IO:       "127.0.0.1",
	IP_KERNEL:   "127.0.0.1",
	PORT_KERNEL: 8001,
	LOG_LEVEL:   "INFO",
}

func main() {
	_, currentFile, _, _ := runtime.Caller(0)
	rutaArchivo = filepath.Dir(currentFile)
	var decision int = 0

	configuracionMemoria.DUMP_PATH = filepath.Join(rutaArchivo, "memoria", "dump")
	configuracionMemoria.SCRIPTS_PATH = filepath.Join(rutaArchivo, "globales", "archivos_prueba")

	for decision != 7 {
		fmt.Println("Preparar entorno para prueba (Escribi el numero): \n  - 1 PLANI CORTO PLAZO \n  - 2 PLANI MYL PLAZO \n  - 3 SWAP \n  - 4 CACHE \n  - 5 TLB \n - 6 ESTABILIDAD GENERAL \n - 7 SALIR")
		fmt.Scan(&decision)
		switch decision {
		case 1:
			var algoritmo int
			fmt.Println("Elegi el algoritmo (Escribi el numero): \n -1 FIFO \n  - 2 SJF \n  - 3 SRT")
			fmt.Scan(&algoritmo)
			switch algoritmo {
			case 1:
				prepararPlaniCortoPlazo("FIFO")
				break
			case 2:
				prepararPlaniCortoPlazo("SJF")
				break
			case 3:
				prepararPlaniCortoPlazo("SRT")
				break
			}

			break
		case 2:
			var algoritmo int
			fmt.Println("Elegi el algoritmo (Escribi el numero): \n -1 FIFO \n  - 2 PMCP")
			fmt.Scan(&algoritmo)
			switch algoritmo {
			case 1:
				prepararPlaniMYLPlazo("FIFO")
				break
			case 2:
				prepararPlaniMYLPlazo("PMCP")
				break
			}
			break
		case 3:
			prepararSWAP()
			break
		case 4:
			var algoritmo int
			fmt.Println("Elegi el algoritmo (Escribi el numero): \n -1 CLOCK \n  - 2 CLOCK-M")
			fmt.Scan(&algoritmo)
			switch algoritmo {
			case 1:
				prepararCACHE("CLOCK")
				break
			case 2:
				prepararCACHE("CLOCK-M")
				break
			}
			break
		case 5:
			var algoritmo int
			fmt.Println("Elegi el algoritmo (Escribi el numero): \n -1 FIFO \n  - 2 LRU")
			fmt.Scan(&algoritmo)
			switch algoritmo {
			case 1:
				prepararTLB("FIFO")
				break
			case 2:
				prepararTLB("LRU")
				break
			}
			break
		case 6:
			var numeroIO int
			var numeroCPU int
			fmt.Println("Cual CPU se va a levantar en esta maquina? (1,2,3,4)")
			fmt.Scan(&numeroCPU)
			fmt.Println("Cual IO se va a levantar en esta maquina? (1,2,3,4)")
			fmt.Scan(&numeroIO)
			prepararEstabilidadGeneral(numeroCPU)
			break
		}
		fmt.Println("\033[H\033[2J")
	}

}

func modificarArchivoCPU(config *ConfigCPU) {
	filePath := filepath.Join(rutaArchivo, "cpu", "config.json")
	//fmt.Println(filePath)

	configFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("no se pudo abrir el config de cpu")
	}

	dataJson, errJson := json.MarshalIndent(config, " ", " ")

	if errJson != nil {
		fmt.Println("no se pudo codificar el config de cpu")
	}

	configFile.Write(dataJson)
	configFile.Close()

}

func modificarArchivoMemoria(config *ConfigMemoria) {
	filePath := filepath.Join(rutaArchivo, "memoria", "config.json")
	//fmt.Println(filePath)

	configFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("no se pudo abrir el config de memoria")
	}

	dataJson, errJson := json.MarshalIndent(config, " ", " ")

	if errJson != nil {
		fmt.Println("no se pudo codificar el config de memoria")
	}

	configFile.Write(dataJson)
	configFile.Close()

}

func modificarArchivoKernel(config *ConfigKernel) {
	filePath := filepath.Join(rutaArchivo, "kernel", "config.json")
	//fmt.Println(filePath)

	configFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("no se pudo abrir el config de kernel")
	}

	dataJson, errJson := json.MarshalIndent(config, " ", " ")

	if errJson != nil {
		fmt.Println("no se pudo codificar el config de kernel")
	}

	configFile.Write(dataJson)
	configFile.Close()

}

func modificarArchivoIO(config *ConfigIO) {
	filePath := filepath.Join(rutaArchivo, "io", "config.json")
	//fmt.Println(filePath)

	configFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("no se pudo abrir el config de io")
	}

	dataJson, errJson := json.MarshalIndent(config, " ", " ")

	if errJson != nil {
		fmt.Println("no se pudo codificar el config de io")
	}

	configFile.Write(dataJson)
	configFile.Close()

}

func prepararEstabilidadGeneral(cpu int) {
	configuracionMemoria.MEMORY_SIZE = 4096
	configuracionMemoria.PAGE_SIZE = 32
	configuracionMemoria.ENTRIES_PER_PAGE = 8
	configuracionMemoria.NUMBER_OF_LEVELS = 3
	configuracionMemoria.MEMORY_DELAY = 100
	configuracionMemoria.SWAP_DELAY = 2500

	go modificarArchivoMemoria(&configuracionMemoria)

	configuracionKernel.SCHEDULER_ALGORITHM = "SRT"
	configuracionKernel.READY_INGRESS_ALGORITHM = "PMCP"
	configuracionKernel.ALPHA = 0.75
	configuracionKernel.INITIAL_ESTIMATE = 100
	configuracionKernel.SUSPENSION_TIME = 3000

	go modificarArchivoKernel(&configuracionKernel)

	switch cpu {
	case 1:
		configuracionCPU.TLB_ENTRIES = 4
		configuracionCPU.TLB_REPLACEMENT = "FIFO"
		configuracionCPU.CACHE_ENTRIES = 2
		configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
		configuracionCPU.CACHE_DELAY = 50
		break
	case 2:
		configuracionCPU.TLB_ENTRIES = 4
		configuracionCPU.TLB_REPLACEMENT = "LRU"
		configuracionCPU.CACHE_ENTRIES = 2
		configuracionCPU.CACHE_REPLACEMENT = "CLOCK-M"
		configuracionCPU.CACHE_DELAY = 50
		break
	case 3:
		configuracionCPU.TLB_ENTRIES = 256
		configuracionCPU.TLB_REPLACEMENT = "FIFO"
		configuracionCPU.CACHE_ENTRIES = 256
		configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
		configuracionCPU.CACHE_DELAY = 1
		break
	case 4:
		configuracionCPU.TLB_ENTRIES = 0
		configuracionCPU.TLB_REPLACEMENT = "FIFO"
		configuracionCPU.CACHE_ENTRIES = 0
		configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
		configuracionCPU.CACHE_DELAY = 0
		break
	}

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}

func prepararTLB(algoritmo string) {
	configuracionCPU.TLB_ENTRIES = 4
	configuracionCPU.TLB_REPLACEMENT = algoritmo
	configuracionCPU.CACHE_ENTRIES = 0
	configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
	configuracionCPU.CACHE_DELAY = 250

	go modificarArchivoCPU(&configuracionCPU)

	configuracionMemoria.MEMORY_SIZE = 2048
	configuracionMemoria.PAGE_SIZE = 32
	configuracionMemoria.ENTRIES_PER_PAGE = 4
	configuracionMemoria.NUMBER_OF_LEVELS = 3
	configuracionMemoria.MEMORY_DELAY = 500
	configuracionMemoria.SWAP_DELAY = 5000

	go modificarArchivoMemoria(&configuracionMemoria)

	configuracionKernel.SCHEDULER_ALGORITHM = "FIFO"
	configuracionKernel.READY_INGRESS_ALGORITHM = "FIFO"
	configuracionKernel.ALPHA = 1
	configuracionKernel.INITIAL_ESTIMATE = 10000
	configuracionKernel.SUSPENSION_TIME = 3000

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}

func prepararCACHE(algoritmo string) {
	configuracionCPU.TLB_ENTRIES = 0
	configuracionCPU.TLB_REPLACEMENT = "FIFO"
	configuracionCPU.CACHE_ENTRIES = 2
	configuracionCPU.CACHE_REPLACEMENT = algoritmo
	configuracionCPU.CACHE_DELAY = 250

	go modificarArchivoCPU(&configuracionCPU)

	configuracionMemoria.MEMORY_SIZE = 2048
	configuracionMemoria.PAGE_SIZE = 32
	configuracionMemoria.ENTRIES_PER_PAGE = 4
	configuracionMemoria.NUMBER_OF_LEVELS = 3
	configuracionMemoria.MEMORY_DELAY = 500
	configuracionMemoria.SWAP_DELAY = 5000

	go modificarArchivoMemoria(&configuracionMemoria)

	configuracionKernel.SCHEDULER_ALGORITHM = "FIFO"
	configuracionKernel.READY_INGRESS_ALGORITHM = "FIFO"
	configuracionKernel.ALPHA = 1
	configuracionKernel.INITIAL_ESTIMATE = 10000
	configuracionKernel.SUSPENSION_TIME = 3000

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}

func prepararSWAP() {
	configuracionCPU.TLB_ENTRIES = 0
	configuracionCPU.TLB_REPLACEMENT = "FIFO"
	configuracionCPU.CACHE_ENTRIES = 0
	configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
	configuracionCPU.CACHE_DELAY = 250

	go modificarArchivoCPU(&configuracionCPU)

	configuracionMemoria.MEMORY_SIZE = 512
	configuracionMemoria.PAGE_SIZE = 32
	configuracionMemoria.ENTRIES_PER_PAGE = 32
	configuracionMemoria.NUMBER_OF_LEVELS = 1
	configuracionMemoria.MEMORY_DELAY = 500
	configuracionMemoria.SWAP_DELAY = 2500

	go modificarArchivoMemoria(&configuracionMemoria)

	configuracionKernel.SCHEDULER_ALGORITHM = "FIFO"
	configuracionKernel.READY_INGRESS_ALGORITHM = "FIFO"
	configuracionKernel.ALPHA = 1
	configuracionKernel.INITIAL_ESTIMATE = 10000
	configuracionKernel.SUSPENSION_TIME = 1000

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}

func prepararPlaniMYLPlazo(algoritmo string) {
	configuracionCPU.TLB_ENTRIES = 4
	configuracionCPU.TLB_REPLACEMENT = "LRU"
	configuracionCPU.CACHE_ENTRIES = 2
	configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
	configuracionCPU.CACHE_DELAY = 250

	go modificarArchivoCPU(&configuracionCPU)

	configuracionMemoria.MEMORY_SIZE = 256
	configuracionMemoria.PAGE_SIZE = 16
	configuracionMemoria.ENTRIES_PER_PAGE = 4
	configuracionMemoria.NUMBER_OF_LEVELS = 2
	configuracionMemoria.MEMORY_DELAY = 500
	configuracionMemoria.SWAP_DELAY = 3000

	go modificarArchivoMemoria(&configuracionMemoria)
	configuracionKernel.SCHEDULER_ALGORITHM = "FIFO"
	configuracionKernel.READY_INGRESS_ALGORITHM = algoritmo
	configuracionKernel.ALPHA = 1
	configuracionKernel.INITIAL_ESTIMATE = 10000
	configuracionKernel.SUSPENSION_TIME = 3000

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}

func prepararPlaniCortoPlazo(algoritmo string) {
	configuracionCPU.TLB_ENTRIES = 4
	configuracionCPU.TLB_REPLACEMENT = "LRU"
	configuracionCPU.CACHE_ENTRIES = 2
	configuracionCPU.CACHE_REPLACEMENT = "CLOCK"
	configuracionCPU.CACHE_DELAY = 250

	go modificarArchivoCPU(&configuracionCPU)

	configuracionMemoria.MEMORY_SIZE = 4096
	configuracionMemoria.PAGE_SIZE = 64
	configuracionMemoria.ENTRIES_PER_PAGE = 4
	configuracionMemoria.NUMBER_OF_LEVELS = 2
	configuracionMemoria.MEMORY_DELAY = 500
	configuracionMemoria.SWAP_DELAY = 15000

	go modificarArchivoMemoria(&configuracionMemoria)
	configuracionKernel.SCHEDULER_ALGORITHM = algoritmo
	configuracionKernel.READY_INGRESS_ALGORITHM = "FIFO"
	configuracionKernel.ALPHA = 1
	configuracionKernel.INITIAL_ESTIMATE = 1000
	configuracionKernel.SUSPENSION_TIME = 120000

	go modificarArchivoKernel(&configuracionKernel)

	go modificarArchivoIO(&configIO)
}
