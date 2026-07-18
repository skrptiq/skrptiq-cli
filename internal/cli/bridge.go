package cli

import (
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/bridge"
)

// Bridge handles `skrptiq bridge <status|enable|disable>` — the browser
// native-messaging bridge lifecycle (GH#866 / K-055). Default-OFF: the bridge
// acts on the user's real browser, so it is never installed or spawned without an
// explicit `enable`.
func Bridge(args []string, dbPath string) int {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}

	engine := OpenEngine(dbPath)
	if engine == nil {
		return ExitFailed
	}
	defer engine.Close()

	mgr := bridge.NewManager(engine.DB)

	switch sub {
	case "status":
		printBridgeStatus(mgr.Status())
		return ExitOK

	case "enable":
		st, err := mgr.Enable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Bridge enabled, but registration reported: %v\n", err)
			printBridgeStatus(st)
			return ExitFailed
		}
		fmt.Println("Browser bridge enabled.")
		printBridgeStatus(st)
		return ExitOK

	case "disable":
		if err := mgr.Disable(); err != nil {
			fmt.Fprintf(os.Stderr, "Error disabling bridge: %v\n", err)
			return ExitFailed
		}
		fmt.Println("Browser bridge disabled.")
		return ExitOK

	default:
		fmt.Fprintln(os.Stderr, "Usage: skrptiq bridge <status|enable|disable>")
		return ExitBadArgs
	}
}

func printBridgeStatus(st bridge.Status) {
	yn := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	fmt.Printf("Browser bridge:\n")
	fmt.Printf("  enabled:        %s\n", yn(st.Enabled))
	fmt.Printf("  host installed: %s\n", yn(st.HostInstalled))
	fmt.Printf("  running:        %s\n", yn(st.Running))
	fmt.Printf("  available:      %s\n", yn(st.Available))
	fmt.Printf("  socket:         %s\n", st.SocketPath)
}
