package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/config"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/execution"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/health"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/httpapi"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/persistence"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/product"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/queue"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/storage"
)

func main() {
	value, err := config.LoadAPI(os.Getenv)
	if err != nil {
		log.Fatalf("invalid API configuration: %v", err)
	}
	provider, err := auth.NewLocalAuthProvider(value.AdminUsername, value.AdminPassword)
	if err != nil {
		log.Fatalf("invalid LocalAuth configuration: %v", err)
	}
	backend, err := loadStorage(os.Getenv)
	if err != nil {
		log.Fatalf("invalid storage configuration: %v", err)
	}
	dsn := strings.TrimSpace(os.Getenv("NH_MEDIA_DATABASE_URL"))
	if dsn == "" {
		log.Fatalf("NH_MEDIA_DATABASE_URL is required; the in-memory Product backend is test-only")
	}
	var productBackend product.Backend
	var runtimeQueue *queue.RedisQueue
	var database *sql.DB
	if dsn != "" {
		if backend == nil {
			log.Fatalf("durable Product API requires NH_STORAGE_ENDPOINT or NH_STORAGE_BACKEND=local")
		}
		database, err = persistence.OpenPostgres(context.Background(), dsn)
		if err != nil {
			log.Fatalf("open Product database: %v", err)
		}
		defer database.Close()
		sqlStore, storeErr := persistence.NewSQLStore(database)
		if storeErr != nil {
			log.Fatalf("initialize Product repository: %v", storeErr)
		}
		bootstrap, bootstrapErr := sqlStore.BootstrapLocalInstallation(context.Background(), persistence.BootstrapInput{Username: value.AdminUsername, DisplayName: "NH-Media Local Admin", PasswordHash: auth.CredentialHash(value.AdminPassword), WorkspaceSlug: firstEnv(os.Getenv, "NH_MEDIA_WORKSPACE_SLUG", "default"), WorkspaceName: "NH-Media Default Workspace"})
		if bootstrapErr != nil {
			log.Fatalf("bootstrap Product identity: %v", bootstrapErr)
		}
		if err := sqlStore.EnsureDefaultWorkflow(context.Background(), bootstrap.UserID); err != nil {
			log.Fatalf("seed native workflow: %v", err)
		}
		if err := provider.ConfigurePrincipal("local-admin", bootstrap.UserID, bootstrap.WorkspaceID, "owner"); err != nil {
			log.Fatalf("configure durable auth principal: %v", err)
		}
		provider.SetPrincipalValidator(func(ctx context.Context, principal auth.Principal) (auth.Principal, error) {
			role, err := sqlStore.RequireWorkspaceRole(ctx, principal.UserID, principal.WorkspaceID)
			if err != nil {
				return auth.Principal{}, err
			}
			principal.Role = role
			return principal, nil
		})
		provider.SetSessionStore(sqlStore)
		productBackend, err = persistence.NewDurableBackend(sqlStore, bootstrap.UserID, bootstrap.WorkspaceID, backend)
		if err != nil {
			log.Fatalf("initialize durable Product backend: %v", err)
		}
	}
	if durable, ok := productBackend.(*persistence.DurableBackend); ok {
		if endpoint := strings.TrimSpace(os.Getenv("NH_QUEUE_ENDPOINT")); endpoint != "" {
			parsed, parseErr := url.Parse(endpoint)
			if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "redis" && parsed.Scheme != "rediss") {
				log.Fatalf("invalid NH_QUEUE_ENDPOINT")
			}
			password := firstEnv(os.Getenv, "NH_QUEUE_PASSWORD", os.Getenv("REDIS_PASSWORD"))
			transport, queueErr := queue.NewRedisQueue(queue.RedisOptions{Addr: parsed.Host, Password: password, StreamPrefix: firstEnv(os.Getenv, "NH_QUEUE_STREAM_PREFIX", "nh-media")})
			if queueErr != nil {
				log.Fatalf("initialize Redis QueuePort: %v", queueErr)
			}
			if err := transport.Ping(context.Background()); err != nil {
				log.Fatalf("connect Redis QueuePort: %v", err)
			}
			defer transport.Close()
			durable.SetQueue(transport)
			runtimeQueue = transport
		}
	}
	server, err := httpapi.NewServerWithBackend(value, provider, health.NewRegistry(nil, 2*time.Second), productBackend, backend)
	if err != nil {
		log.Fatalf("invalid API server: %v", err)
	}
	if durable, ok := productBackend.(*persistence.DurableBackend); ok && runtimeQueue != nil {
		workerToken := strings.TrimSpace(os.Getenv("NH_MEDIA_WORKER_TOKEN"))
		if len(workerToken) < 24 {
			log.Fatalf("NH_MEDIA_WORKER_TOKEN must contain at least 24 characters when workers are enabled")
		}
		server.ConfigureWorkerArtifacts(persistence.WorkerTransfer{SQL: durable.SQL, Storage: backend, Workspace: durable.Workspace}, workerToken)
	}
	dispatchCtx, stopDispatch := context.WithCancel(context.Background())
	defer stopDispatch()
	if durable, ok := productBackend.(*persistence.DurableBackend); ok && runtimeQueue != nil {
		claims := execution.NewLeaseRegistry()
		artifactRepository, repositoryErr := persistence.NewSQLArtifactRepository(durable.SQL, durable.UserID, durable.Workspace)
		if repositoryErr != nil {
			log.Fatalf("initialize worker Artifact repository: %v", repositoryErr)
		}
		for _, capability := range []string{"probe", "thumbnail", "analysis", "ai", "ml", "media", "render", "system"} {
			workerID := "api-controller-" + capability
			controller := execution.ClaimingDispatcher{Queue: runtimeQueue, Resolver: persistence.ScopedClaimResolver{SQL: durable.SQL, Workspace: durable.Workspace, WorkerID: workerID, Duration: 2 * time.Minute}, Publisher: runtimeQueue, Claims: claims}
			results := execution.LeaseAwareResultReconciler{Source: runtimeQueue, Claims: claims, Applier: persistence.ScopedResultApplier{SQL: durable.SQL, Workspace: durable.Workspace, WorkerID: workerID, Artifacts: persistence.WorkerArtifactCommitter{Storage: backend, Repository: artifactRepository}}}
			go func(capability string, controller execution.ClaimingDispatcher) {
				for dispatchCtx.Err() == nil {
					if err := controller.Run(dispatchCtx, capability, "api-controller-"+capability, 250*time.Millisecond); err != nil && dispatchCtx.Err() == nil {
						log.Printf("execution dispatcher %s stopped: %v", capability, err)
						time.Sleep(time.Second)
					}
				}
			}(capability, controller)
			go func(capability string, reconciler execution.LeaseAwareResultReconciler) {
				for dispatchCtx.Err() == nil {
					if err := reconciler.Run(dispatchCtx, capability, "api-result-"+capability, 250*time.Millisecond); err != nil && dispatchCtx.Err() == nil {
						log.Printf("worker result reconciler %s stopped: %v", capability, err)
						time.Sleep(time.Second)
					}
				}
			}(capability, results)
		}
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				if _, err := durable.ScheduleReadySteps(dispatchCtx); err != nil && dispatchCtx.Err() == nil {
					log.Printf("dependency scheduler stopped: %v", err)
				}
				if _, err := durable.RequeueRetryingSteps(dispatchCtx); err != nil && dispatchCtx.Err() == nil {
					log.Printf("retry sweeper stopped: %v", err)
				}
				select {
				case <-dispatchCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	go func() {
		log.Printf("NH-Media API shell listening on %s (profile=%s)", value.Bind, value.Profile)
		if err := server.HTTPServer.ListenAndServe(); err != nil {
			log.Printf("API stopped: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	stopDispatch()
	server.Health.SetDraining(true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.HTTPServer.Shutdown(ctx); err != nil {
		log.Printf("API drain failed: %v", err)
	}
}

func firstEnv(env func(string) string, name, fallback string) string {
	if value := strings.TrimSpace(env(name)); value != "" {
		return value
	}
	return fallback
}

func loadStorage(env func(string) string) (storage.StoragePort, error) {
	backend := strings.ToLower(strings.TrimSpace(env("NH_STORAGE_BACKEND")))
	if backend != "" && backend != "local" && backend != "s3" && backend != "minio" {
		return nil, fmt.Errorf("unsupported NH_STORAGE_BACKEND %q", backend)
	}
	if backend == "local" {
		root := strings.TrimSpace(env("NH_STORAGE_LOCAL_ROOT"))
		if root == "" {
			return nil, fmt.Errorf("NH_STORAGE_LOCAL_ROOT is required for local storage")
		}
		return storage.NewLocalStorage(root, strings.TrimSpace(env("NH_STORAGE_CACHE_ROOT")))
	}
	endpointRaw := strings.TrimSpace(env("NH_STORAGE_ENDPOINT"))
	if endpointRaw == "" {
		if backend == "s3" || backend == "minio" {
			return nil, fmt.Errorf("NH_STORAGE_ENDPOINT is required for %s storage", backend)
		}
		return nil, nil
	}
	accessKey := strings.TrimSpace(env("NH_STORAGE_ACCESS_KEY"))
	secretKey := strings.TrimSpace(env("NH_STORAGE_SECRET_KEY"))
	bucket := strings.TrimSpace(env("NH_STORAGE_BUCKET"))
	if accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("NH_STORAGE_ACCESS_KEY, NH_STORAGE_SECRET_KEY and NH_STORAGE_BUCKET are required")
	}
	parsed, err := url.Parse(endpointRaw)
	if err != nil {
		return nil, fmt.Errorf("NH_STORAGE_ENDPOINT: %w", err)
	}
	endpoint := parsed.Host
	secure := parsed.Scheme == "https"
	if endpoint == "" {
		endpoint = parsed.Path
	}
	if endpoint == "" {
		return nil, fmt.Errorf("NH_STORAGE_ENDPOINT has no host")
	}
	return storage.NewS3Storage(endpoint, accessKey, secretKey, bucket, secure)
}
