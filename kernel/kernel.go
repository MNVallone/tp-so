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
	mux.HandleFunc("/cpu/paquete", utils.AtenderCPU)
	mux.HandleFunc("/cpu/handshake", utils.AtenderHandshakeCPU)
	mux.HandleFunc("/cpu/retorno", utils.AtenderRetornoCPU)
	mux.HandleFunc("/io/paquete", servidor.RecibirPaquetesIO)
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

	// inicializo map de estimaciones
	utils.EstimacionProcesos = make(map[int]float64)

	// validacion de argumentos para proceso inicial
	if len(os.Args) < 2 {
		fmt.Println("Error: Falta el archivo de pseudocódigo")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("Error: Falta el tamaño del proceso")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	rutaInicial := os.Args[1]
	tamanio, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: El tamaño del proceso debe ser un número entero")
		os.Exit(1)
	}

	// creo proceso inicial
	utils.CrearProceso(rutaInicial, tamanio)

	// espero enter para iniciar planificador
	slog.Info("Presione ENTER para iniciar el planificador...")

	// uso un buffer para leer el enter
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
