package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"redis-clone/internal/server"
)

func main() {
	addr := flag.String("addr", ":6379", "TCP address to listen on")
	snapshotPath := flag.String("snapshot", "dump.json", "path to the persistence snapshot file")
	snapshotInterval := flag.Duration("snapshot-interval", 30*time.Second, "how often to auto-save a snapshot")
	flag.Parse()

	srv := server.New(*addr, *snapshotPath)

if err := srv.LoadSnapshot(); err != nil {
		// A missing snapshot file is not an error here (LoadFromFile
		// already returns nil for that case) — reaching this branch
		// means the file exists but is corrupt/unreadable. Starting
		// anyway would mean running with a silently empty store, and
		// the next save (periodic or on shutdown) would then
		// overwrite the corrupt-but-possibly-partially-recoverable
		// file with an empty one, destroying it for good. Refuse to
		// start instead — same philosophy real Redis uses for a
		// corrupt AOF/RDB file: fail loudly, require a human to fix
		// or remove the bad snapshot before proceeding.
		log.Fatalf("could not load snapshot %q: %v — fix or remove this file to start with an empty store", *snapshotPath, err)
	}

	srv.StartPeriodicSnapshot(*snapshotInterval)

	// On SIGINT (Ctrl+C) or SIGTERM (what most process managers send
	// for a graceful stop), close the listener so ListenAndServe
	// unblocks below — that's our cue to save one final snapshot
	// before the process actually exits. Note: SIGKILL (kill -9)
	// cannot be caught by any program, ever — the periodic snapshot
	// above is the backstop for that case, not this handler.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received %s, shutting down", sig)
		if err := srv.Close(); err != nil {
			log.Printf("error closing listener: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server exited: %v", err)
	}

	log.Println("saving final snapshot before exit")
	if err := srv.SaveSnapshot(); err != nil {
		log.Printf("error saving final snapshot: %v", err)
	}
}
