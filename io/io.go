package main

import (
	"fmt"
	"globales"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	utils "github.com/sisoputnfrba/tp-golang/io/utils"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	
	var rutaConfig string

		if len(os.Args) < 2 {
	slog.Error("No se ha pasado el nombre del archivo de configuracion")
		fmt.Println("Uso: io <config_file>")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("Error: Debe especificar el nombre del dispositivo IO")
		fmt.Println("Uso: ./bin/io [nombre]")
		os.Exit(1)
	}

	utils.NombreDispositivo = os.Args[2]

	dir, _ := filepath.Abs(".")

	// Obtiene la ruta del directorio padre
	//parentDir := filepath.Dir(dir)

	rutaConfig = filepath.Join(dir, "configs", os.Args[1])


	utils.ClientConfig = utils.IniciarConfiguracion(rutaConfig)

	// ------ LOGGING ------ //
	globales.ConfigurarLogger(fmt.Sprintf("io_%s.log", utils.NombreDispositivo), utils.ClientConfig.LOG_LEVEL)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
		os.Exit(1)
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_kernel := utils.ClientConfig.PORT_KERNEL
	ip_kernel := utils.ClientConfig.IP_KERNEL
	puerto_io := ":" + strconv.Itoa(utils.ClientConfig.PORT_IO)

	// ------ REGISTRO DE SEÑALES ------ //
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux := http.NewServeMux()
	mux.HandleFunc("/io/peticion", utils.AtenderPeticionIO)

	go escucharPeticiones(puerto_io, mux)

	// ------ HANDSHAKE CON KERNEL ------ //
	utils.RealizarHandshake(ip_kernel, puerto_kernel)

	slog.Info(fmt.Sprintf("Dispositivo IO '%s' iniciado y listo para recibir peticiones", utils.NombreDispositivo))

	// Esperar señal para terminar
	<-sigChan

	respuesta := utils.RespuestaIO{
		PID:                utils.PIDActual,
		Motivo:             "Desconexion",
		Nombre_Dispositivo: utils.NombreDispositivo,
		IP:                 utils.ClientConfig.IP_IO,
		Puerto:             utils.ClientConfig.PORT_IO,
	}

	// contesto al kernel
	globales.GenerarYEnviarPaquete(&respuesta, ip_kernel, puerto_kernel, "/io/finalizado")

	slog.Info(fmt.Sprintf("Cerrando dispositivo IO '%s'...", utils.NombreDispositivo))
}

func escucharPeticiones(puerto_io string, mux *http.ServeMux) {
	slog.Info(fmt.Sprintf("Iniciando dispositivo IO '%s' en el puerto %s", utils.NombreDispositivo, puerto_io))
	err := http.ListenAndServe(puerto_io, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		os.Exit(1)
	}
}
