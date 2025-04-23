package utils

// Si el nombre de una funcion/variable empieza con una letra mayuscula, es porque es exportable
// Si empieza con una letra minuscula, es porque es privada al paquete

import (
	"encoding/json"
	"fmt"
	"globales"
	"log/slog"
	"os"
)

type Config struct {
	IP_MEMORY           string `json:"ip_memory"`
	PORT_MEMORY         int    `json:"port_memory"`
	PORT_KERNEL         int    `json:"port_kernel"`
	SCHEDULER_ALGORITHM string `json:"sheduler_algorithm"`
	NEW_ALGORITHM       string `json:"new_algorithm"`
	ALPHA               int    `json:"alpha"`
	SUSPENSION_TIME     int    `json:"suspension_time"`
	LOG_LEVEL           string `json:"log_level"`
}

var ClientConfig *Config

type Paquete struct {
	Valores string `json:"valores"`
}

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		slog.Error(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// Colas de estado de los procesos
var ColaNew []globales.PCB
var ColaReady []globales.PCB
var ColaRunning []globales.PCB
var ColaBlocked []globales.PCB
var ColaSuspendedBlocked []globales.PCB
var ColaSuspendedReady []globales.PCB
var ColaExit []globales.PCB

//TODO: implementar semaforo para modificar colas de PCBs

func AgregarPCBaCola(pcb globales.PCB, cola *[]globales.PCB) {
	//cola.Lock()
	//defer cola.Unlock()
	*cola = append(*cola, pcb)
	slog.Info(fmt.Sprintf("globales.PCB agregado a la cola: %v", pcb))
}

func EliminarPCBaCola(pcb globales.PCB, cola *[]globales.PCB) {
	// cola.Lock()
	// defer cola.Unlock()
	for i, p := range *cola {
		if p.PID == pcb.PID {
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			slog.Info(fmt.Sprintf("PCB eliminado de la cola: %v", pcb))
			return
		}
	}
	slog.Info(fmt.Sprintf("PCB no encontrado en la cola: %v", pcb))
}
