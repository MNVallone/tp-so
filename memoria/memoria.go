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
	globales.ConfigurarLogger("memoria.log", utils.ClientConfig.LOG_LEVEL)
	slog.Info("Iniciando módulo Memoria", "puerto", utils.ClientConfig.PORT_MEMORY)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)

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
