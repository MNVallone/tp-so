package main

import (
	"globales"
	"globales/servidor"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sisoputnfrba/tp-golang/io/utils" // = "io/utils"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("io.log", utils.ClientConfig.LOG_LEVEL)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_kernel := utils.ClientConfig.PORT_KERNEL
	ip_kernel := "localhost" //utils.ClientConfig.IP_KERNEL
	// puerto_io := ":" + strconv.Itoa(utils.ClientConfig.PORT_IO)

	// ------ INICIALIZACION DE CLIENTE ------ //
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	unPaquete := servidor.Paquete{
		Valores : []string{"Ana", "Luis", "Pedro"},
		UnNumero: 42,
	}

	globales.GenerarYEnviarPaquete(&unPaquete, ip_kernel, puerto_kernel, "/io/paquete")

	<-sigChan 

	slog.Info("Cerrando modulo IO ...")
}
