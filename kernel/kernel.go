package main

import (
	"bufio"
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
	_ "time"
)

func main() {
	utils.InicializarColas()

	// ------ VALIDACION DE ARGUMENTOS ------ //
	rutaInicial, tamanio := utils.ValidarArgumentosKernel()

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("kernel.log", utils.ClientConfig.LOG_LEVEL)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES LOCALES ------ //
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	puerto_kernel := ":" + strconv.Itoa(utils.ClientConfig.PORT_KERNEL)
	ip_memoria := utils.ClientConfig.IP_MEMORY
	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux.HandleFunc("/cpu/paquete", utils.AtenderCPU)            //TODO: implementar para CPU
	mux.HandleFunc("/cpu/handshake", utils.AtenderHandshakeCPU) // TODO: implementar con semaforo para que no haya CC
	mux.HandleFunc("/cpu/interrupt", utils.AtenderCPU)
	mux.HandleFunc("/cpu/solicitarIO", utils.SolicitarIO)         // syscall IO
	mux.HandleFunc("/cpu/iniciarProceso", utils.IniciarProceso)   // syscall INIT_PROC
	mux.HandleFunc("/cpu/terminarProceso", utils.TerminarProceso) // syscall EXIT
	mux.HandleFunc("/cpu/dumpearMemoria", utils.DumpearMemoria)   // syscall DUMP_MEMORY
	mux.HandleFunc("/io/paquete", servidor.RecibirPaquetesIO)     //TODO: implementar para IO
	mux.HandleFunc("/io/handshake", utils.AtenderHandshakeIO)
	//mux.HandleFunc("/io/finalizado", utils.AtenderFinIOPeticion)
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

	utils.CrearProceso(rutaInicial, tamanio) // creo el proceso inicial
	utils.CrearProceso(rutaInicial, 10) // creo el proceso inicial
	utils.CrearProceso(rutaInicial, 100) // creo el proceso inicial

	slog.Info("Presione ENTER para iniciar el planificador...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	// inicio planificador
	slog.Info("Iniciando planificadores...")
	utils.IniciarPlanificadores()

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
