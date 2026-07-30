// Package models descobre os GGUF já baixados na máquina.
//
// A varredura cobre o layout do cache do Hugging Face usado pelo llama.cpp
// (models--<org>--<repo>/snapshots/<sha>/*.gguf) e também GGUF soltos.
package models

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type Source int

const (
	// SourceHF: modelo no layout de cache do Hugging Face, pode subir com -hf.
	SourceHF Source = iota
	// SourceLocal: GGUF solto, sobe com -m.
	SourceLocal
)

type Model struct {
	Source     Source
	Repo       string // org/repo, apenas para SourceHF
	Quant      string
	Path       string
	Size       int64
	Shards     int
	MTPPath    string
	MMProjPath string
	Root       string
}

// ID é o identificador exibido e persistido no perfil.
func (m Model) ID() string {
	if m.Source == SourceHF {
		if m.Quant != "" {
			return m.Repo + ":" + m.Quant
		}
		return m.Repo
	}
	return m.Path
}

// Title é o nome curto mostrado na lista.
func (m Model) Title() string {
	if m.Source == SourceHF {
		return m.Repo
	}
	return filepath.Base(m.Path)
}

func (m Model) HasMTP() bool { return m.MTPPath != "" }

func (m Model) HasMMProj() bool { return m.MMProjPath != "" }

// Args devolve os argumentos que apontam o llama-server para este modelo.
// Com useLocalPath o modelo do cache HF sobe por caminho absoluto, o que evita
// qualquer acesso a rede mas desliga a busca automática do sidecar MTP.
func (m Model) Args(useLocalPath bool) []string {
	if m.Source == SourceLocal || useLocalPath {
		return []string{"-m", m.Path}
	}
	return []string{"-hf", m.ID()}
}

func (m Model) SizeHuman() string { return HumanSize(m.Size) }

func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

// Roots devolve os diretórios varridos, na ordem de precedência.
func Roots(extra []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		if strings.HasPrefix(p, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, strings.TrimPrefix(p, "~"))
			}
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return
		}
		if seen[abs] {
			return
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}

	add(os.Getenv("LLAMA_CACHE"))
	add(os.Getenv("LLAMA_MODELS"))
	for _, p := range extra {
		add(p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if runtime.GOOS == "darwin" {
			add(filepath.Join(home, "Library", "Caches", "llama.cpp"))
		}
		add(filepath.Join(home, ".cache", "llama.cpp"))
		add(filepath.Join(home, "models"))
	}
	return out
}

var (
	shardRe = regexp.MustCompile(`-(\d{5})-of-(\d{5})$`)
	repoRe  = regexp.MustCompile(`^models--(.+?)--(.+)$`)
)

// Scan varre os roots e devolve os modelos encontrados, ordenados por nome.
func Scan(extra []string) ([]Model, []string) {
	roots := Roots(extra)
	var found []Model
	var warns []string

	for _, root := range roots {
		files, err := collectGGUF(root)
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", root, err))
			continue
		}
		found = append(found, classify(root, files)...)
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].Source != found[j].Source {
			return found[i].Source < found[j].Source
		}
		return strings.ToLower(found[i].ID()) < strings.ToLower(found[j].ID())
	})

	// modelos identicos vistos por roots diferentes (ex: symlink) aparecem uma vez
	out := found[:0]
	seen := map[string]bool{}
	for _, m := range found {
		key := m.Source.String() + "|" + m.ID()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out, warns
}

func (s Source) String() string {
	if s == SourceHF {
		return "hf"
	}
	return "local"
}

type ggufFile struct {
	path string
	size int64
}

func collectGGUF(root string) ([]ggufFile, error) {
	var out []ggufFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// blobs guarda os arquivos reais por hash, os snapshots já apontam para lá
			if d.Name() == "blobs" || d.Name() == "refs" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".gguf") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size := info.Size()
		if info.Mode()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(p); err == nil {
				size = st.Size()
			}
		}
		out = append(out, ggufFile{path: p, size: size})
		return nil
	})
	return out, err
}

func classify(root string, files []ggufFile) []Model {
	// agrupa por diretório: num snapshot HF os sidecars ficam ao lado do modelo
	byDir := map[string][]ggufFile{}
	for _, f := range files {
		dir := filepath.Dir(f.path)
		byDir[dir] = append(byDir[dir], f)
	}

	var out []Model
	for dir, group := range byDir {
		repo := repoOf(dir)
		sizes := map[string]int64{}
		shards := map[string]int{}
		var primaries []string
		var mtp, mmproj string

		for _, f := range group {
			base := strings.TrimSuffix(filepath.Base(f.path), filepath.Ext(f.path))
			lower := strings.ToLower(base)
			switch {
			case strings.HasPrefix(lower, "mtp-"):
				mtp = f.path
				continue
			case strings.HasPrefix(lower, "mmproj"):
				mmproj = f.path
				continue
			}

			stem := base
			shard := 1
			if m := shardRe.FindStringSubmatch(base); m != nil {
				stem = strings.TrimSuffix(base, m[0])
				if m[1] != "00001" {
					shard = 0 // shard secundário: soma o tamanho mas não vira entrada
				}
			}
			sizes[stem] += f.size
			shards[stem]++
			if shard == 1 {
				primaries = append(primaries, stem+"|"+f.path)
			}
		}

		sort.Strings(primaries)
		for _, entry := range primaries {
			i := strings.Index(entry, "|")
			stem, path := entry[:i], entry[i+1:]
			m := Model{
				Path:       path,
				Size:       sizes[stem],
				Shards:     shards[stem],
				MTPPath:    mtp,
				MMProjPath: mmproj,
				Root:       root,
				Quant:      quantOf(stem),
			}
			if repo != "" {
				m.Source = SourceHF
				m.Repo = repo
			} else {
				m.Source = SourceLocal
				m.MTPPath = "" // fora do layout HF o sidecar não é resolvido pelo -hf
			}
			out = append(out, m)
		}
	}
	return out
}

// repoOf extrai org/repo do caminho de um snapshot do cache HF.
func repoOf(dir string) string {
	for p := dir; ; p = filepath.Dir(p) {
		if m := repoRe.FindStringSubmatch(filepath.Base(p)); m != nil {
			return m[1] + "/" + m[2]
		}
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
	}
}

// quantOf infere o quant pelo sufixo do nome do arquivo, como o llama.cpp faz.
func quantOf(stem string) string {
	i := strings.LastIndex(stem, "-")
	if i < 0 || i == len(stem)-1 {
		return ""
	}
	q := stem[i+1:]
	if len(q) > 12 {
		return ""
	}
	return q
}
