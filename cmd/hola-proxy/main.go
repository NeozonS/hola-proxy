package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	xproxy "golang.org/x/net/proxy"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/dns"
	"github.com/NeozonS/hola-proxy/internal/hola"
	applog "github.com/NeozonS/hola-proxy/internal/log"
	"github.com/NeozonS/hola-proxy/internal/proxy"
	"github.com/NeozonS/hola-proxy/internal/surfclient"
	"github.com/NeozonS/hola-proxy/internal/tunnel"
	"github.com/NeozonS/hola-proxy/internal/version"
)

const HolaExtStoreID = "gkojfkhlekighikafcpjkiklfbnlmeio"

// endpointProbeTarget is CONNECT'ed through each candidate zagent at startup.
// Timing that handshake (not a bulk download) is enough to rank agents by
// setup latency; the proxy typically lives only a few hours per process.
const endpointProbeTarget = "1.1.1.1:443"
const endpointProbeTimeout = 8 * time.Second

// PROTOCOL_WHITELIST gates which agents are emitted by -list-proxies. Hola
// returns a "protocol" tag per agent; we only support the plain HTTP variant.
var PROTOCOL_WHITELIST = map[string]bool{
	"HTTP": true,
	"http": true,
}

// appVersion is set at build time via -ldflags="-X main.appVersion=...".
var appVersion = "undefined"

func perror(msg string) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, msg)
}

func argFail(msg string) {
	perror(msg)
	perror("Usage:")
	flag.PrintDefaults()
	os.Exit(2)
}

type CLIArgs struct {
	extVer                               string
	country                              string
	listCountries, listProxies, useTrial bool
	limit                                uint
	bindAddress                          string
	verbosity                            int
	timeout, rotate                      time.Duration
	proxyType                            string
	resolver                             string
	bootstrapDNS                         string
	forcePortField                       string
	showVersion                          bool
	proxy                                string
	caFile                               string
	backoffInitial                       time.Duration
	backoffDeadline                      time.Duration
	initRetries                          int
	initRetryInterval                    time.Duration
	hideSNI                              bool
	userAgent                            *string
	banCooldown                          time.Duration
	logFile                              string
}

func parseArgs() CLIArgs {
	var args CLIArgs
	flag.StringVar(&args.extVer, "ext-ver", "", "extension version to mimic in requests. "+
		"Can be obtained from https://chrome.google.com/webstore/detail/hola-vpn-the-website-unbl/gkojfkhlekighikafcpjkiklfbnlmeio")
	flag.StringVar(&args.forcePortField, "force-port-field", "", "force specific port field/num (example 24232 or lum)")
	flag.StringVar(&args.country, "country", "us", "desired proxy location")
	flag.BoolVar(&args.listCountries, "list-countries", false, "list available countries and exit")
	flag.BoolVar(&args.listProxies, "list-proxies", false, "output proxy list and exit")
	flag.UintVar(&args.limit, "limit", 3, "amount of proxies in retrieved list")
	flag.StringVar(&args.bindAddress, "bind-address", "127.0.0.1:8080", "HTTP proxy listen address")
	flag.IntVar(&args.verbosity, "verbosity", 20, "logging verbosity "+
		"(10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical)")
	flag.DurationVar(&args.timeout, "timeout", 35*time.Second, "timeout for network operations")
	flag.DurationVar(&args.rotate, "rotate", 48*time.Hour, "rotate user ID once per given period")
	flag.DurationVar(&args.backoffInitial, "backoff-initial", 3*time.Second, "initial average backoff delay for zgettunnels (randomized by +/-50%)")
	flag.DurationVar(&args.backoffDeadline, "backoff-deadline", 5*time.Minute, "total duration of zgettunnels method attempts")
	flag.IntVar(&args.initRetries, "init-retries", 0, "number of attempts for initialization steps, zero for unlimited retry")
	flag.DurationVar(&args.initRetryInterval, "init-retry-interval", 30*time.Second, "delay between initialization retries")
	flag.DurationVar(&args.banCooldown, "ban-cooldown", time.Hour,
		"cooldown between Hola API attempts after a temporary ban (randomized +/-50%). "+
			"Hammering the API while banned only extends the ban")
	flag.StringVar(&args.logFile, "log-file", "", "append log output to this file (in addition to stderr)")
	flag.StringVar(&args.proxyType, "proxy-type", "direct", "proxy type: direct or lum")
	flag.StringVar(&args.resolver, "resolver", "https://cloudflare-dns.com/dns-query",
		"DNS/DoH/DoT resolver to workaround Hola blocked hosts. "+
			"See https://github.com/ameshkov/dnslookup/ for upstream DNS URL format.")
	flag.StringVar(&args.bootstrapDNS, "bootstrap-dns", "",
		"override the system DNS server used for bootstrap lookups (e.g. \"8.8.8.8:53\"). "+
			"Useful on platforms like Termux where Go can't read /etc/resolv.conf and "+
			"falls back to an unreachable [::1]:53. Empty means use the system resolver.")
	flag.BoolVar(&args.useTrial, "dont-use-trial", false, "use regular ports instead of trial ports")
	flag.BoolVar(&args.showVersion, "version", false, "show program version and exit")
	flag.StringVar(&args.proxy, "proxy", "", "sets base proxy to use for all dial-outs. "+
		"Format: <http|https|socks5|socks5h>://[login:password@]host[:port] "+
		"Examples: http://user:password@192.168.1.1:3128, socks5://10.0.0.1:1080")
	flag.StringVar(&args.caFile, "cafile", "", "use custom CA certificate bundle file")
	flag.Func("user-agent",
		"override User-Agent. Default: Windows Chrome UA matching the TLS impersonate profile (do not set a newer Chrome than the JA3)",
		func(s string) error {
			args.userAgent = &s
			return nil
		})
	flag.BoolVar(&args.hideSNI, "hide-SNI", true, "hide SNI in TLS sessions with proxy server")
	flag.Parse()
	if args.country == "" {
		argFail("Country can't be empty string.")
	}
	if args.proxyType == "" {
		argFail("Proxy type can't be an empty string.")
	}
	if args.listCountries && args.listProxies {
		argFail("list-countries and list-proxies flags are mutually exclusive")
	}
	return args
}

func run() int {
	args := parseArgs()
	if args.showVersion {
		fmt.Println(appVersion)
		return 0
	}

	// On some platforms (notably Termux) Go's standard resolver can't read
	// /etc/resolv.conf and falls back to an unreachable [::1]:53, breaking every
	// bootstrap lookup with "connection refused". -bootstrap-dns forces the
	// pure-Go resolver to dial a known-good server instead.
	if args.bootstrapDNS != "" {
		dnsAddr := args.bootstrapDNS
		if _, _, err := net.SplitHostPort(dnsAddr); err != nil {
			dnsAddr = net.JoinHostPort(dnsAddr, "53")
		}
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 10 * time.Second}
				return d.DialContext(ctx, network, dnsAddr)
			},
		}
	}

	var logOut io.Writer = os.Stderr
	if args.logFile != "" {
		f, err := os.OpenFile(args.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't open log file %q: %v\n", args.logFile, err)
			return 14
		}
		defer f.Close()
		logOut = io.MultiWriter(os.Stderr, f)
	}

	logWriter := applog.NewLogWriter(logOut)
	defer logWriter.Close()

	mainLogger := applog.NewCondLogger(log.New(logWriter, "MAIN    : ", log.LstdFlags|log.Lshortfile), args.verbosity)
	credLogger := applog.NewCondLogger(log.New(logWriter, "CRED    : ", log.LstdFlags|log.Lshortfile), args.verbosity)
	proxyLogger := applog.NewCondLogger(log.New(logWriter, "PROXY   : ", log.LstdFlags|log.Lshortfile), args.verbosity)

	var dialer core.ContextDialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	var caPool *x509.CertPool
	if args.caFile != "" {
		caPool = x509.NewCertPool()
		certs, err := ioutil.ReadFile(args.caFile)
		if err != nil {
			mainLogger.Error("Can't load CA file: %v", err)
			return 15
		}
		if ok := caPool.AppendCertsFromPEM(certs); !ok {
			mainLogger.Error("Can't load certificates from CA file")
			return 15
		}
		_ = tls.Config{RootCAs: caPool} // sanity check; consumed via hola.Config below
	}

	proxyFromURLWrapper := func(u *url.URL, next xproxy.Dialer) (xproxy.Dialer, error) {
		cdialer, ok := next.(core.ContextDialer)
		if !ok {
			return nil, errors.New("only context dialers are accepted")
		}
		return tunnel.ProxyDialerFromURL(u, caPool, cdialer)
	}

	if args.proxy != "" {
		xproxy.RegisterDialerType("http", proxyFromURLWrapper)
		xproxy.RegisterDialerType("https", proxyFromURLWrapper)
		proxyURL, err := url.Parse(args.proxy)
		if err != nil {
			mainLogger.Critical("Unable to parse base proxy URL: %v", err)
			return 6
		}
		pxDialer, err := xproxy.FromURL(proxyURL, dialer)
		if err != nil {
			mainLogger.Critical("Unable to instantiate base proxy dialer: %v", err)
			return 7
		}
		dialer = pxDialer.(core.ContextDialer)
	}

	try := retryPolicy(args.initRetries, args.initRetryInterval, mainLogger)

	// list-countries doesn't need Chrome/extension version detection.
	if args.listCountries {
		client := hola.NewClient(hola.Config{
			Dialer:  dialer,
			RootCAs: caPool,
		})
		return printCountries(client, try, args.timeout)
	}

	mainLogger.Info("hola-proxy client version %s is starting...", appVersion)

	// Fail fast on a -country Hola doesn't serve instead of retrying an
	// empty zgettunnels response forever. The catalog lookup doesn't need
	// the impersonated client, so a lightweight one is enough here.
	if err := validateCountry(hola.NewClient(hola.Config{Dialer: dialer, RootCAs: caPool}), try, mainLogger, args.country, args.timeout); err != nil {
		mainLogger.Critical("%v", err)
		pauseOnError(mainLogger)
		return 9
	}

	userAgent := surfclient.ChromeWindowsUA()
	if args.userAgent != nil {
		userAgent = *args.userAgent
		mainLogger.Warning("overriding impersonate User-Agent with %q; TLS/sec-ch-ua still follow Chrome %s",
			userAgent, surfclient.ChromeProdVersion())
	} else {
		mainLogger.Info("using impersonate User-Agent: %q", userAgent)
	}

	if args.extVer == "" {
		err := try("get latest version of browser extension", func() error {
			ctx, cl := context.WithTimeout(context.Background(), args.timeout)
			defer cl()
			prod := surfclient.ChromeProdVersion()
			extVer, err := version.GetExtVer(ctx, &prod, HolaExtStoreID, dialer)
			if err == nil {
				mainLogger.Info("discovered latest browser extension version: %s", extVer)
				args.extVer = extVer
			}
			return err
		})
		if err != nil {
			mainLogger.Critical("Can't detect latest browser extension version. Try to specify -ext-ver parameter. Error: %v", err)
			return 8
		}
		mainLogger.Warning("Detected latest extension version: %q. Pass -ext-ver parameter to skip resolve and speedup startup", args.extVer)
	}

	client := hola.NewClient(hola.Config{
		UserAgent: userAgent,
		Dialer:    dialer,
		RootCAs:   caPool,
		ExtVer:    args.extVer,
	})

	if args.listProxies {
		return printProxies(client, try, mainLogger, args.country, args.proxyType, args.limit, args.timeout, args.backoffInitial, args.backoffDeadline)
	}

	mainLogger.Info("Constructing fallback DNS upstream...")
	resolver, err := dns.NewResolver(args.resolver, args.timeout)
	if err != nil {
		mainLogger.Critical("Unable to instantiate DNS resolver: %v", err)
		return 6
	}

	var (
		auth    core.AuthProvider
		tunnels *hola.ZGetTunnelsResponse
	)
	// Bind the listen address BEFORE talking to the Hola API. If another
	// instance is already running on this address, we exit immediately —
	// without minting a new user UUID first. Parallel instances registering
	// fresh identities from the same IP are a primary ban trigger.
	listener, err := net.Listen("tcp", args.bindAddress)
	if err != nil {
		mainLogger.Critical("Can't listen on %s: %v. Is another instance of hola-proxy already running?", args.bindAddress, err)
		return 16
	}

	err = try("run credentials service", func() error {
		auth, tunnels, err = hola.CredService(client, args.rotate, args.timeout, args.country, args.proxyType, credLogger, args.backoffInitial, args.backoffDeadline, args.banCooldown)
		return err
	})
	if err != nil {
		if errors.Is(err, hola.PermanentBanError) {
			mainLogger.Critical("Hola permanently banned this IP/identity. Options: wait it out, " +
				"route the API through another proxy with -proxy, or change your external IP address.")
		}
		return 4
	}

	endpoints, err := hola.Endpoints(tunnels, args.proxyType, args.useTrial, args.forcePortField)
	if err != nil {
		mainLogger.Critical("Unable to determine proxy endpoint: %v", err)
		return 5
	}

	probeTimeout := endpointProbeTimeout
	if args.timeout > 0 && args.timeout < probeTimeout {
		probeTimeout = args.timeout
	}
	probeCtx, probeCancel := context.WithTimeout(context.Background(), probeTimeout)
	mainLogger.Info("Probing %d endpoints via CONNECT %s (timeout %s)...",
		len(endpoints), endpointProbeTarget, probeTimeout)
	endpoint, err := hola.PickFastest(probeCtx, mainLogger, endpoints, func(ctx context.Context, ep *hola.Endpoint) (time.Duration, error) {
		d := tunnel.NewProxyDialer(ep.NetAddr(), ep.TLSName, caPool, auth, args.hideSNI, dialer)
		start := time.Now()
		conn, derr := d.DialContext(ctx, "tcp", endpointProbeTarget)
		if derr != nil {
			return 0, derr
		}
		conn.Close()
		return time.Since(start), nil
	})
	probeCancel()
	if err != nil {
		mainLogger.Critical("Unable to pick a working endpoint: %v", err)
		return 5
	}

	handlerDialer := tunnel.NewProxyDialer(endpoint.NetAddr(), endpoint.TLSName, caPool, auth, args.hideSNI, dialer)
	requestDialer := tunnel.NewPlaintextDialer(endpoint.NetAddr(), endpoint.TLSName, caPool, args.hideSNI, dialer)
	mainLogger.Info("Endpoint: %s", endpoint.URL().String())
	mainLogger.Info("Starting proxy server...")

	handler := proxy.NewHandler(handlerDialer, requestDialer, auth, resolver, proxyLogger)
	mainLogger.Info("Init complete.")
	err = http.Serve(listener, handler)
	mainLogger.Critical("Server terminated with a reason: %v", err)
	mainLogger.Info("Shutting down...")
	return 0
}

func main() {
	os.Exit(run())
}

// pauseOnError keeps the console window open for a minute after a fatal
// startup error: when the binary is launched by double-clicking, the window
// would otherwise close before the message can be read. Ctrl+C exits sooner.
func pauseOnError(logger *applog.CondLogger) {
	logger.Warning("Exiting in a minute so the error stays readable; press Ctrl+C to quit sooner...")
	time.Sleep(time.Minute)
}

func retryPolicy(retries int, retryInterval time.Duration, logger *applog.CondLogger) func(string, func() error) error {
	return func(name string, f func() error) error {
		var err error
		for i := 1; retries <= 0 || i <= retries; i++ {
			if i > 1 {
				logger.Warning("Retrying action %q in %v...", name, retryInterval)
				time.Sleep(retryInterval)
			}
			logger.Info("Attempting action %q, attempt #%d...", name, i)
			err = f()
			if err == nil {
				logger.Info("Action %q succeeded on attempt #%d", name, i)
				return nil
			}
			if errors.Is(err, hola.PermanentBanError) {
				logger.Critical("Action %q failed with a permanent ban; not retrying", name)
				return err
			}
			if errors.Is(err, hola.TemporaryBanError) {
				// The long-running proxy path never reaches this (it paces
				// ban retries with a cooldown internally). One-shot actions
				// like -list-proxies must not hammer the API with fresh
				// identities on every retry — that only extends the ban.
				logger.Critical("Action %q failed with a temporary ban; not retrying. "+
					"Wait for the ban to lift before trying again", name)
				return err
			}
			logger.Warning("Action %q failed: %v", name, err)
		}
		logger.Critical("All attempts for action %q have failed. Last error: %v", name, err)
		return err
	}
}
