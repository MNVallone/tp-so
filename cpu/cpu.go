package main

import (
	"cpu/utils"
	"fmt"
	"globales"
	"globales/servidor"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	idCpu := "1" // default si no se pasa argumento
	if len(os.Args) > 1 {
		idCpu = os.Args[1]
	}

	logFileName := fmt.Sprintf("cpu-%s.log", idCpu)

	// ------ LOGGING ------ //
	// globales.ConfigurarLogger("cpu.log", utils.ClientConfig.LOG_LEVEL) // configurar logger
	globales.ConfigurarLogger(logFileName, utils.ClientConfig.LOG_LEVEL) // configurar logger

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto := ":" + strconv.Itoa(utils.ClientConfig.PORT_CPU)
	ip_memoria := utils.ClientConfig.IP_MEMORY
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	ip_kernel := utils.ClientConfig.IP_KERNEL
	puerto_kernel := utils.ClientConfig.PORT_KERNEL

	//var urlBase string = fmt.Sprintf("/cpu/%s/handshake", idCpu)

	//mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	//mux.HandleFunc((urlBase + "/handshake")), utils.AtenderCPU) //TODO: implementar para CPU

	slog.Info(fmt.Sprintf("El puerto es %s", puerto))

	// ------ INICIALIZACION DEL CLIENTE ------ //
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	pcb := servidor.PCB{
		PID: 1120,
		ESTADO : "Hola desde el cpu",
		ESPACIO_EN_MEMORIA : 1024,
	}

	handshakeCPU := globales.HandshakeCPU{
		ID_CPU: idCpu,
		PORT_CPU: 8080,
		IP_CPU: "127.1.1.0",
	}
	
	globales.GenerarYEnviarPaquete(&handshakeCPU, ip_kernel, puerto_kernel, "/cpu/handshake")

	utils.IO("jose", 3000)
	utils.INIT_PROC("archivo.txt", 3000)

	globales.GenerarYEnviarPaquete(&pcb, ip_memoria, puerto_memoria, "/cpu/paquete")
	// globales.GenerarYEnviarPaquete(&mensaje, ip_memoria, puerto_memoria, "/kernel/paqueteKernel")

	<-sigChan 

	slog.Info("Cerrando modulo CPU ...")
}
