package main

import (
	"cpu/utils"
	"fmt"
	"globales"
	"globales/servidor"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	utils.IdCpu = "1" // default si no se pasa argumento
	if len(os.Args) > 1 {
		utils.IdCpu = os.Args[1]
	}

	logFileName := fmt.Sprintf("cpu-%s.log", utils.IdCpu)

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

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	//mux.HandleFunc((urlBase + "/handshake")), utils.AtenderCPU) //TODO: implementar para CPU
	mux.HandleFunc(fmt.Sprintf("/cpu/%s/ejecutarProceso", utils.IdCpu), utils.EjecutarProceso) //TODO: implementar para CPU

	mux.HandleFunc("/kernel/interrupt", utils.CHECK_INTERRUPT)

	slog.Info(fmt.Sprintf("El puerto es %s", puerto))

	// ------ INICIALIZACION DEL CLIENTE ------ //
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	pcb := servidor.PCB{
		PID:                1120,
		ESTADO:             "Hola desde el cpu",
		ESPACIO_EN_MEMORIA: 1024,
	}

	handshakeCPU := globales.HandshakeCPU{
		ID_CPU:   utils.IdCpu,
		PORT_CPU: utils.ClientConfig.PORT_CPU, // 8004
		IP_CPU:   utils.ClientConfig.IP_CPU,
	}

	/* Esto esta para probar multiples conexiones de cpu desde la misma pc
	if (idCpu == "1"){
		handshakeCPU = globales.HandshakeCPU{
			ID_CPU: idCpu,
			PORT_CPU: utils.ClientConfig.PORT_CPU, // 8004
			IP_CPU: utils.ClientConfig.IP_CPU,
		}
	}

	if (idCpu == "2"){
		handshakeCPU = globales.HandshakeCPU{
			ID_CPU: idCpu,
			PORT_CPU: 8005,
			IP_CPU: utils.ClientConfig.IP_CPU,
		}
		puerto = ":8005"
	}
	*/

	go escucharPeticiones(puerto, mux)

	globales.GenerarYEnviarPaquete(&handshakeCPU, ip_kernel, puerto_kernel, "/cpu/handshake")

	//utils.IO("jose", 3000)
	//utils.INIT_PROC("archivo.txt", 3000)

	globales.GenerarYEnviarPaquete(&pcb, ip_memoria, puerto_memoria, "/cpu/paquete")
	// globales.GenerarYEnviarPaquete(&mensaje, ip_memoria, puerto_memoria, "/kernel/paqueteKernel")

	utils.WRITE(0, "Entro la balubi")

	utils.READ(0, 15)

	<-sigChan

	slog.Info("Cerrando modulo CPU ...")
}

func escucharPeticiones(puerto string, mux *http.ServeMux) {
	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}
}
