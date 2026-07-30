// Package config persiste os perfis de execucao do llama-server.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juliosouzam/llamadeck/internal/catalog"
)

// ParamValue e o estado de um parametro no perfil.
type ParamValue struct {
	Enabled bool   `json:"enabled"`
	Value   string `json:"value,omitempty"`
}

// ModelRef aponta para o modelo escolhido sem depender do resultado da varredura.
type ModelRef struct {
	Repo         string `json:"repo,omitempty"`
	Quant        string `json:"quant,omitempty"`
	Path         string `json:"path,omitempty"`
	UseLocalPath bool   `json:"use_local_path,omitempty"`
}

func (r ModelRef) Empty() bool { return r.Repo == "" && r.Path == "" }

func (r ModelRef) ID() string {
	if r.Repo != "" {
		if r.Quant != "" {
			return r.Repo + ":" + r.Quant
		}
		return r.Repo
	}
	return r.Path
}

// Args devolve -hf ou -m conforme a preferencia gravada no perfil.
func (r ModelRef) Args() []string {
	if r.Repo != "" && !r.UseLocalPath {
		return []string{"-hf", r.ID()}
	}
	if r.Path != "" {
		return []string{"-m", r.Path}
	}
	if r.Repo != "" {
		return []string{"-hf", r.ID()}
	}
	return nil
}

// Profile e um conjunto nomeado de modelo + parametros.
type Profile struct {
	Name   string                `json:"name"`
	Model  ModelRef              `json:"model"`
	Params map[string]ParamValue `json:"params"`
	Env    map[string]string     `json:"env,omitempty"`
}

func (p Profile) Clone() Profile {
	out := p
	out.Params = make(map[string]ParamValue, len(p.Params))
	for k, v := range p.Params {
		out.Params[k] = v
	}
	if p.Env != nil {
		out.Env = make(map[string]string, len(p.Env))
		for k, v := range p.Env {
			out.Env[k] = v
		}
	}
	return out
}

// Args monta a linha de comando completa do llama-server para este perfil.
func (p Profile) Args() []string {
	args := append([]string{}, p.Model.Args()...)
	for _, g := range catalog.Groups {
		for _, s := range g.Specs {
			v, ok := p.Params[s.ID]
			if !ok || !v.Enabled {
				continue
			}
			args = append(args, s.Args(v.Value)...)
		}
	}
	return args
}

// Endpoint devolve a URL usada no health check e mostrada no cabecalho.
func (p Profile) Endpoint() string {
	host := "127.0.0.1"
	port := "8080"
	if v, ok := p.Params["host"]; ok && v.Enabled && strings.TrimSpace(v.Value) != "" {
		host = strings.TrimSpace(v.Value)
	}
	if v, ok := p.Params["port"]; ok && v.Enabled && strings.TrimSpace(v.Value) != "" {
		port = strings.TrimSpace(v.Value)
	}
	if strings.HasSuffix(host, ".sock") {
		return ""
	}
	switch host {
	case "0.0.0.0", "":
		host = "127.0.0.1"
	case "::":
		host = "[::1]"
	}
	scheme := "http"
	if v, ok := p.Params["ssl-cert-file"]; ok && v.Enabled && strings.TrimSpace(v.Value) != "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

// EnvPairs devolve o Env do perfil no formato KEY=VALUE.
func (p Profile) EnvPairs() []string {
	keys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+p.Env[k])
	}
	return out
}

// Config e o arquivo persistido em ~/.config/llamadeck/config.json.
type Config struct {
	Binary    string   `json:"binary"`
	ExtraDirs []string `json:"extra_dirs,omitempty"`
	// KeepArgEnv repassa as LLAMA_ARG_* do shell para o servidor. Desligado por
	// padrao para que so os parametros marcados na TUI tenham efeito.
	KeepArgEnv bool      `json:"keep_arg_env,omitempty"`
	Current    Profile   `json:"current"`
	Profiles   []Profile `json:"profiles"`
}

func Path() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "llamadeck", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		dir, _ := os.UserConfigDir()
		return filepath.Join(dir, "llamadeck", "config.json")
	}
	return filepath.Join(home, ".config", "llamadeck", "config.json")
}

// DefaultBinary procura o llama-server no PATH e cai para caminhos conhecidos.
func DefaultBinary() string {
	if p, err := exec.LookPath("llama-server"); err == nil {
		return p
	}
	for _, c := range []string{"/opt/homebrew/bin/llama-server", "/usr/local/bin/llama-server"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return "llama-server"
}

// DefaultProfile traz um ponto de partida seguro para Apple Silicon.
func DefaultProfile() Profile {
	p := Profile{Name: "atual", Params: map[string]ParamValue{}}
	for _, id := range []string{"host", "port", "ctx-size", "cache-type-k", "cache-type-v", "n-gpu-layers", "jinja", "metrics"} {
		s, ok := catalog.Lookup(id)
		if !ok {
			continue
		}
		p.Params[id] = ParamValue{Enabled: true, Value: s.Default}
	}
	set := func(id, v string) {
		pv := p.Params[id]
		pv.Enabled = true
		pv.Value = v
		p.Params[id] = pv
	}
	set("ctx-size", "32768")
	set("cache-type-k", "q8_0")
	set("cache-type-v", "q8_0")
	set("n-gpu-layers", "99")
	return p
}

func Default() *Config {
	return &Config{
		Binary:   DefaultBinary(),
		Current:  DefaultProfile(),
		Profiles: []Profile{},
	}
}

// Load le a configuracao do disco. Um arquivo ausente devolve os defaults.
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return Default(), fmt.Errorf("config invalida em %s: %w", Path(), err)
	}
	if cfg.Binary == "" {
		cfg.Binary = DefaultBinary()
	}
	if cfg.Current.Params == nil {
		cfg.Current = DefaultProfile()
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Params == nil {
			cfg.Profiles[i].Params = map[string]ParamValue{}
		}
	}
	return cfg, nil
}

func (c *Config) Save() error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// FindProfile devolve o indice do perfil com o nome dado, ou -1.
func (c *Config) FindProfile(name string) int {
	for i, p := range c.Profiles {
		if strings.EqualFold(p.Name, name) {
			return i
		}
	}
	return -1
}

// UpsertProfile grava o perfil, substituindo um homonimo se existir.
func (c *Config) UpsertProfile(p Profile) {
	if i := c.FindProfile(p.Name); i >= 0 {
		c.Profiles[i] = p
		return
	}
	c.Profiles = append(c.Profiles, p)
}

func (c *Config) DeleteProfile(i int) {
	if i < 0 || i >= len(c.Profiles) {
		return
	}
	c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
}

// Shell devolve a linha de comando pronta para colar num terminal.
func Shell(binary string, args []string) string {
	var b strings.Builder
	b.WriteString(quote(binary))
	for _, a := range args {
		b.WriteString(" ")
		b.WriteString(quote(a))
	}
	return b.String()
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\n\"'\\$`&|<>()*?;#") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}
