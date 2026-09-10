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
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
)

const defaultPort = 3010

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
	serveCmd.Flags().IntVar(&servePort, "port", defaultPort, "port to listen on")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "do not open the browser")
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := resolveDataDir()
	if err != nil {
		return err
	}
	db, err := sqlite.Open(ctx, dbPath(dir))
	if err != nil {
		return err
	}
	defer db.Close()

	handler, err := httpadapter.NewHandler()
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(servePort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	url := "http://localhost:" + strconv.Itoa(servePort)
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
