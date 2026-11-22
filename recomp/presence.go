package main

import (
	"log"
	"sync"
	"time"

	"github.com/hugolgst/rich-go/client"
)

const discordClientID = "1499447238923915274"

type presenceUpdate struct {
	details string
	state   string
}

type DiscordPresence struct {
	updates chan presenceUpdate
	done    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

func NewDiscordPresence() *DiscordPresence {
	presence := &DiscordPresence{
		updates: make(chan presenceUpdate, 1),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go presence.run()
	return presence
}

func (p *DiscordPresence) Update(details, state string) {
	update := presenceUpdate{details: details, state: state}
	select {
	case p.updates <- update:
	default:
		select {
		case <-p.updates:
		default:
		}
		select {
		case p.updates <- update:
		default:
		}
	}
}

func (p *DiscordPresence) Close() {
	p.once.Do(func() { close(p.done) })
	<-p.stopped
}

func (p *DiscordPresence) run() {
	defer close(p.stopped)
	if err := client.Login(discordClientID); err != nil {
		log.Printf("Discord Rich Presence unavailable: %v", err)
		return
	}
	defer client.Logout()

	started := time.Now()
	var current presenceUpdate
	setActivity := func(update presenceUpdate) {
		current = update
		if err := client.SetActivity(client.Activity{
			Details:    update.details,
			State:      update.state,
			LargeImage: "logo",
			LargeText:  "Fastroads",
			Timestamps: &client.Timestamps{Start: &started},
		}); err != nil {
			log.Printf("update Discord Rich Presence: %v", err)
		}
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case update := <-p.updates:
			if update != current {
				setActivity(update)
			}
		case <-ticker.C:
			if current.details != "" {
				setActivity(current)
			}
		case <-p.done:
			return
		}
	}
}
