package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"globales"
	"globales/servidor"
	"io"
	"log/slog"
	"memoria/utils"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

func main() {
	// ------ CONFIGURACIONES ------ //

	_, currentFile, _, _ := runtime.Caller(0)    // devuelve ruta absoluta del .go actual
	utils.RutaModulo = filepath.Dir(currentFile) // obtiene el directorio del archivo

	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	utils.InicializarMemoria()

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log", utils.ClientConfig.LOG_niveles)
	slog.Info("Iniciando módulo Memoria", "puerto", utils.ClientConfig.PORT_MEMORY)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)
	//log_level := utils.ClientConfig.LOG_LEVEL

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //

	// Eliminar estas funciones o implementarlas como handshake.
	mux.HandleFunc("/cpu/paquete", utils.AtenderCPU)                  // TODO: implementar para CPU
	mux.HandleFunc("/kernel/paquete", servidor.RecibirPaquetesKernel) // TODO: implementar para Kernel

	mux.HandleFunc("/kernel/inicializar_proceso", utils.InicializarProceso)
	mux.HandleFunc("/kernel/suspender_proceso", utils.SuspenderProceso)
	mux.HandleFunc("/kernel/dessuspender_proceso", utils.DesSuspenderProceso)
	mux.HandleFunc("/kernel/finalizar_proceso", utils.FinalizarProceso)
	mux.HandleFunc("/kernel/dump_de_proceso", utils.DumpearProceso)

	mux.HandleFunc("/cpu/leer_pagina", utils.LeerPaginaCompleta)
	mux.HandleFunc("/cpu/buscar_instruccion", utils.DevolverInstruccion)
	mux.HandleFunc("/cpu/leer_direccion", utils.LeerDireccion)
	mux.HandleFunc("/cpu/escribir_direccion", utils.EscribirDireccion)
	mux.HandleFunc("/cpu/obtener_marco", utils.ObtenerMarco)
	mux.HandleFunc("/cpu/escribir_pagina", utils.EscribirPaginaCompleta)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	/*
			TablaPaginas := utils.CrearTablaPaginas(1, utils.ClientConfig.NUMBER_OF_LEVELS, utils.ClientConfig.ENTRIES_PER_PAGE)
			TablaPaginas2 := utils.CrearTablaPaginas(1, utils.ClientConfig.NUMBER_OF_LEVELS, utils.ClientConfig.ENTRIES_PER_PAGE)

			utils.ReservarMemoria(2115, TablaPaginas)
			utils.ReservarMemoria(1000, TablaPaginas2)

			var marcosAsignados1 []int
			var marcosAsignados2 []int

			utils.ObtenerMarcosAsignados(TablaPaginas, 1, &marcosAsignados1)
			utils.ObtenerMarcosAsignados(TablaPaginas2, 1, &marcosAsignados2)
			fmt.Println("Los marcos asignados al proceso son: ")
			fmt.Println(marcosAsignados1)
			fmt.Println(marcosAsignados2)

			indices := []int{0, 0, 1}                         // Indices para acceder a la tabla de paginas
			utils.ObtenerMarcoDeTDP(TablaPaginas, indices, 1) // Acceder al primer marco de memoria del proceso 1

			utils.DesasignarMarcos(TablaPaginas2, 1)
			utils.DesasignarMarcos(TablaPaginas, 1)
			fmt.Println("Marcos libres: ", utils.MarcosLibres)

		utils.Crear_procesoPrueba(1024, 1)
		utils.SuspenderProcesoPrueba(1)
	*/
	go escucharPeticiones(puerto_memoria, mux)

	<-sigChan // Esperar a recibir una señal
	//slog.Debug(fmt.Sprintf("Memoria contigua: %x ", utils.MemoriaDeUsuario))
	//DebugSwapCompleto()
	slog.Info("Cerrando modulo memoria ...")
}

func escucharPeticiones(puerto string, mux *http.ServeMux) {
	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}
}

func DebugSwapCompleto() {

	rutaSwap := filepath.Join(utils.RutaModulo, utils.ClientConfig.SWAPFILE_PATH)
	swapfile, err := os.OpenFile(rutaSwap, os.O_RDONLY, 0644)
	if err != nil {
		slog.Error(fmt.Sprintf("Error abriendo swap para debug: %v", err))
		return
	}
	defer swapfile.Close()

	contenido, err := io.ReadAll(swapfile)
	if err != nil {
		slog.Error(fmt.Sprintf("Error leyendo swap para debug: %v", err))
		return
	}

	buffer := bytes.NewBuffer(contenido)
	decoder := gob.NewDecoder(buffer)

	slog.Info("== DEBUG CONTENIDO ACTUAL DE SWAP ==")
	i := 0
	for {
		var proceso utils.ProcesoSwap
		err := decoder.Decode(&proceso)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error(fmt.Sprintf("Error decodificando proceso #%d: %v", i, err))
			break
		}
		slog.Info(fmt.Sprintf("Proceso #%d - PID: %d - Data: %v", i, proceso.PID, proceso.Data))
		i++
	}
	slog.Info("== FIN DEBUG SWAP ==")
}
