package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	httpadapter "github.com/melvicsosa/skillman/internal/adapters/http"
	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

var (
	servePort   int
	serveNoOpen bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local HTTP server and open the UI",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 0, "port to listen on (default: the port setting, "+strconv.Itoa(app.DefaultPort)+")")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "do not open the browser")
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rt, err := newRuntime(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()
	dir := rt.dataDir

	// First run: populate the cache so the UI has something to show.
	if skills, err := rt.svc.ListSkills(ctx, domain.SkillFilter{}); err != nil {
		return err
	} else if len(skills) == 0 {
		sum, err := rt.svc.Scan(ctx, app.ScanOptions{})
		if err != nil {
			return err
		}
		printScanSummary(cmd, sum)
	}

	handler, err := httpadapter.NewHandler(rt.svc)
	if err != nil {
		return err
	}

	port := servePort
	if !cmd.Flags().Changed("port") {
		if port, err = rt.svc.Port(ctx); err != nil {
			return err
		}
	} else if err := app.ValidatePort(port); err != nil {
		return err
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return portBusyError(ctx, port, rt.svc.PIDFile())
		}
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	if err := os.WriteFile(rt.svc.PIDFile(), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "could not write pid file: %v\n", err)
	}
	defer os.Remove(rt.svc.PIDFile())

	url := "http://localhost:" + strconv.Itoa(port)
	fmt.Fprintf(cmd.OutOrStdout(), "skillman listening on %s (data dir %s)\n", url, dir)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	if !serveNoOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "could not open browser: %v\n", err)
		}
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// portBusyError explains who holds the port when it is a skillman server.
func portBusyError(ctx context.Context, port int, pidFile string) error {
	healthy, pid := app.ProbeHealth(ctx, port)
	if pid == 0 {
		pid = app.ReadPIDFile(pidFile)
	}
	if !healthy {
		return fmt.Errorf("port %d is already in use by another program", port)
	}
	who := ""
	if pid > 0 {
		who = fmt.Sprintf(" (pid %d)", pid)
	}
	return fmt.Errorf("skillman is already running on :%d%s. Use `skillman service status` or open http://localhost:%d", port, who, port)
}

// openBrowser opens url with the platform default handler.
func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "linux":
		c = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	return c.Start()
}
