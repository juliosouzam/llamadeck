// Command llamadeck é uma TUI para configurar e operar o llama-server.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juliosouzam/llamadeck/internal/config"
	"github.com/juliosouzam/llamadeck/internal/models"
	"github.com/juliosouzam/llamadeck/internal/server"
	"github.com/juliosouzam/llamadeck/internal/ui"
)

const version = "0.1.0"

func main() {
	var (
		profileName = flag.String("profile", "", "carrega um perfil salvo antes de abrir a TUI")
		printCmd    = flag.Bool("print", false, "imprime o comando do llama-server e sai")
		listModels  = flag.Bool("list-models", false, "lista os modelos encontrados e sai")
		showVersion = flag.Bool("version", false, "mostra a versão e sai")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("llamadeck", version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
	}

	if *profileName != "" {
		i := cfg.FindProfile(*profileName)
		if i < 0 {
			fmt.Fprintf(os.Stderr, "perfil %q não encontrado\n", *profileName)
			os.Exit(1)
		}
		cfg.Current = cfg.Profiles[i].Clone()
	}

	if *listModels {
		found, warns := models.Scan(cfg.ExtraDirs)
		for _, w := range warns {
			fmt.Fprintln(os.Stderr, "aviso:", w)
		}
		for _, m := range found {
			badges := ""
			if m.HasMTP() {
				badges += " [MTP]"
			}
			if m.HasMMProj() {
				badges += " [mmproj]"
			}
			fmt.Printf("%-60s %10s%s\n", m.ID(), m.SizeHuman(), badges)
		}
		return
	}

	if *printCmd {
		fmt.Println(config.Shell(cfg.Binary, cfg.Current.Args()))
		return
	}

	mgr := server.New(8000)
	app := ui.New(cfg, mgr)

	// sem mouse tracking: preserva a seleção de texto do terminal para copiar logs
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro na TUI:", err)
		os.Exit(1)
	}

	st := mgr.State()
	switch {
	case app.StopOnExit():
		mgr.StopAndWait(10 * time.Second)
		fmt.Println("servidor parado.")
	case st.Status.Active():
		fmt.Printf("servidor continua rodando (pid %d) em %s\n", st.PID, cfg.Current.Endpoint())
		fmt.Printf("para parar: kill %d\n", st.PID)
	}

	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "aviso: não consegui salvar a config:", err)
	}
}
