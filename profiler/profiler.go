package profiler

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/google/pprof/driver"
	"github.com/google/pprof/profile"
)

type SubProfiler interface {
	Start() error
	Stop() error
}

type Profiler struct {
	baseDir         string
	mux             *http.ServeMux
	lastIdx         int
	served          bool
	pprofFile       *os.File
	redirectOnce    sync.Once
	redirectHandler *redirectHandler
	subProfilers    []SubProfiler
}

type redirectHandler struct {
	lastIdx int
}

func (h *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirectURL := fmt.Sprintf("http://%s/%d%s", r.Host, h.lastIdx, r.URL.Path)
	http.Redirect(w, r, redirectURL, http.StatusFound) // prevent browser cache
}

func NewProfiler(baseDir string) *Profiler {
	return &Profiler{
		baseDir:         baseDir,
		mux:             http.NewServeMux(),
		redirectHandler: &redirectHandler{},
	}
}

func (p *Profiler) AddProfiler(profiler SubProfiler) {
	p.subProfilers = append(p.subProfilers, profiler)
}

func (p *Profiler) createBaseDirIfNotExists() error {
	if p.baseDir == "" {
		p.baseDir = os.TempDir()
	} else {
		if err := os.MkdirAll(p.baseDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", p.baseDir, err)
		}
	}
	return nil
}

func (p *Profiler) ListenAndServe(port uint16) error {
	if err := p.createBaseDirIfNotExists(); err != nil {
		return err
	}
	filePath := []string{}
	filepath.Walk(p.baseDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".pprof" {
			return nil
		}
		filePath = append(filePath, path)
		return nil
	})
	for _, path := range filePath {
		if err := p.addProfileResult(path); err != nil {
			return fmt.Errorf("failed to add profile result: %w", err)
		}
	}
	p.served = true
	return http.ListenAndServe(fmt.Sprintf(":%d", port), p.mux)
}

func (p *Profiler) addProfileResult(pprofPath string) error {
	file, err := os.Open(pprofPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	pprof, err := profile.Parse(file)
	if err != nil {
		return fmt.Errorf("failed to parse pprof: %w", err)
	}
	p.redirectHandler.lastIdx = p.lastIdx
	options := &driver.Options{
		Fetch:   &fetcher{pprof: pprof},
		UI:      new(ui),
		Flagset: new(flagSet),
		HTTPServer: func(args *driver.HTTPServerArgs) error {
			p.redirectOnce.Do(func() {
				for route := range args.Handlers {
					p.mux.Handle(route, p.redirectHandler)
				}
			})
			for route, handler := range args.Handlers {
				trimmed := strings.TrimLeft(route, "/")
				route = fmt.Sprintf("/%d/%s", p.lastIdx, trimmed)
				p.mux.Handle(route, handler)
			}
			return nil
		},
	}
	if err := driver.PProf(options); err != nil {
		return fmt.Errorf("failed to run pprof: %w", err)
	}
	p.lastIdx++
	return nil
}

const (
	fileFormat = "2006_01_02_15_04_05"
)

func (p *Profiler) Start() error {
	if err := p.createBaseDirIfNotExists(); err != nil {
		return err
	}
	currentTime := time.Now().Format(fileFormat)
	pprofFileName := fmt.Sprintf("pprof_%s.pprof", currentTime)
	pprofFilePath := filepath.Join(p.baseDir, pprofFileName)
	f, err := os.Create(pprofFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", pprofFilePath, err)
	}
	p.pprofFile = f
	log.Printf("start pprof: report to %s", pprofFilePath)
	pprof.StartCPUProfile(f)
	for _, sub := range p.subProfilers {
		if err := sub.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Profiler) Stop() error {
	pprof.StopCPUProfile()
	if p.pprofFile != nil {
		p.pprofFile.Close()
	}
	if p.served {
		if err := p.addProfileResult(p.pprofFile.Name()); err != nil {
			return fmt.Errorf("failed to add profile result: %w", err)
		}
	}
	for idx, sub := range p.subProfilers {
		if err := sub.Stop(); err != nil {
			log.Printf("failed to stop profiler%d: %+v", idx, err)
		}
	}
	return nil
}

type fetcher struct {
	pprof *profile.Profile
}

func (f *fetcher) Fetch(src string, duration, timeout time.Duration) (*profile.Profile, string, error) {
	return f.pprof, "", nil
}

type flagSet struct{}

func (s *flagSet) Bool(name string, def bool, usage string) *bool {
	var v bool
	return &v
}
func (s *flagSet) Int(name string, def int, usage string) *int {
	var v int
	return &v
}
func (s *flagSet) Float64(name string, def float64, usage string) *float64 {
	var v float64 = 1
	return &v
}
func (s *flagSet) String(name string, def string, usage string) *string {
	if name == "http" {
		v := "0.0.0.0:0"
		return &v
	}
	var v string
	return &v
}
func (s *flagSet) StringList(name string, def string, usage string) *[]*string {
	var v []*string
	return &v
}
func (s *flagSet) ExtraUsage() string {
	return ""
}

func (s *flagSet) AddExtraUsage(eu string) {
}

func (s *flagSet) Parse(usage func()) []string {
	return []string{"-http", "0.0.0.0:0"}
}

type ui struct{}

func (*ui) ReadLine(prompt string) (string, error)       { return "", nil }
func (*ui) Print(...interface{})                         {}
func (*ui) PrintErr(...interface{})                      {}
func (*ui) IsTerminal() bool                             { return false }
func (*ui) WantBrowser() bool                            { return false }
func (*ui) SetAutoComplete(complete func(string) string) {}
