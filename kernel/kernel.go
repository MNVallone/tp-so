package main

import (
	"fmt"
	"globales"
	"globales/servidor"
	"kernel/utils"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("kernel.log", utils.ClientConfig.LOG_LEVEL)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	puerto_kernel := ":" + strconv.Itoa(utils.ClientConfig.PORT_KERNEL)
	ip_memoria := utils.ClientConfig.IP_MEMORY

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux.HandleFunc("/cpu/paquete", utils.AtenderCPU) //TODO: implementar para CPU
	mux.HandleFunc("/cpu/handshake", utils.AtenderHandshakeCPU)
	mux.HandleFunc("/io/paquete", servidor.RecibirPaquetesIO)   //TODO: implementar para IO
	mux.HandleFunc("/io/handshake", utils.AtenderHandshakeIO)
	mux.HandleFunc("/io/finalizado", utils.AtenderFinIOPeticion)
	// Manejar señales para terminar el programa de forma ordenada
	sigChan := make(chan os.Signal, 1)                      // canal para recibir señales
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM) //Le dice al programa que cuando reciba una señal del tipo SIGINT o SIGTERM la envíe al canal.

	go escucharPeticiones(puerto_kernel, mux)

	slog.Info(fmt.Sprintf("Servidor escuchando en el puerto %s", puerto_kernel))

	// ------ INICIALIZACION DEL CLIENTE ------ //
	mensaje := servidor.Mensaje{
		Mensaje: "Hola desde el kernel",
	}

	unPCB := globales.PCB{
		PID: 1,
		PC:  0,
		ME: globales.METRICAS_KERNEL{
			NEW:               0,
			READY:             0,
			RUNNING:           0,
			BLOCKED:           0,
			SUSPENDED_BLOCKED: 0,
			SUSPENDED_READY:   0,
			EXIT:              0,
		},
		MT: globales.METRICAS_KERNEL{
			NEW:               0,
			READY:             0,
			RUNNING:           0,
			BLOCKED:           0,
			SUSPENDED_BLOCKED: 0,
			SUSPENDED_READY:   0,
			EXIT:              0,
		},
	}

	utils.AgregarPCBaCola(unPCB, &utils.ColaNew)

	//utils.LeerPCBDesdeCola(&utils.ColaNew)

	time.Sleep(500)

	utils.CambiarDeEstado(&utils.ColaNew, &utils.ColaReady)

	globales.GenerarYEnviarPaquete(&mensaje, ip_memoria, puerto_memoria, "/kernel/paquete")

	<-sigChan // Esperar a recibir una señal

	slog.Info("Cerrando modulo Kernel ...")
}

func escucharPeticiones(puerto string, mux *http.ServeMux) {
	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}
}
