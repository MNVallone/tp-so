package main

import (
	"globales"
	"log"
	"github.com/sisoputnfrba/tp-golang/io/utils" // = "io/utils"
	"globales/servidor"
)

func main() {
	// ------ LOGGING ------ //
	globales.ConfigurarLogger("io.log")

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	if utils.ClientConfig == nil {
		log.Fatalf("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_kernel := utils.ClientConfig.PORT_KERNEL
	ip_kernel := "localhost" //utils.ClientConfig.IP_KERNEL
	// puerto_io := ":" + strconv.Itoa(utils.ClientConfig.PORT_IO)

	// ------ INICIALIZACION DE CLIENTE ------ //
	//mensaje := "Hola desde el IO"

	unPaquete := servidor.Paquete{
		Valores : []string{"Ana", "Luis", "Pedro"},
		UnNumero: 42,
	}

	globales.GenerarYEnviarPaquete(&unPaquete, ip_kernel, puerto_kernel)
}
