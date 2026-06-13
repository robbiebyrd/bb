package playback

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

// PlaybackCanClient replays a log file as CAN frames, preserving the original
// inter-message timing. Set loop=true to restart from the beginning once the
// file is exhausted.
//
// The URI field (options.URI in CanInterfaceOption) is the path to the log
// file. Format detection is automatic — see DetectParser.
type PlaybackCanClient struct {
	ctx         context.Context
	id          int
	name        string
	network     string
	uri         string // log file path
	channel     chan canModels.CanMessageTimestamped
	opened      bool
	streaming   bool
	loop        bool
	l           *slog.Logger
	cfg         *canModels.Config
	dbcFilePath *string
}

// NewPlaybackCanClient creates a PlaybackCanClient.
// filePath is the log file to replay; it is also used as the URI when uri is nil.
func NewPlaybackCanClient(
	ctx context.Context,
	cfg *canModels.Config,
	name string,
	channel chan canModels.CanMessageTimestamped,
	filePath string,
	loop bool,
	logger *slog.Logger,
	network, uri, dbcFilePath *string,
) *PlaybackCanClient {
	if name == "" {
		panic(fmt.Errorf("connection name cannot be empty"))
	}
	if channel == nil {
		panic(fmt.Errorf("message channel cannot be nil"))
	}

	if network == nil || *network == "" {
		defaultNetwork := "playback"
		network = &defaultNetwork
	}

	// URI defaults to the file path when not explicitly set.
	resolvedURI := filePath
	if uri != nil && *uri != "" {
		resolvedURI = *uri
	}

	return &PlaybackCanClient{
		ctx:         ctx,
		name:        name,
		channel:     channel,
		network:     *network,
		uri:         resolvedURI,
		loop:        loop,
		l:           logger,
		cfg:         cfg,
		dbcFilePath: dbcFilePath,
	}
}

func (c *PlaybackCanClient) GetID() int               { return c.id }
func (c *PlaybackCanClient) SetID(id int)             { c.id = id }
func (c *PlaybackCanClient) GetName() string          { return c.name }
func (c *PlaybackCanClient) SetName(name string)      { c.name = name }
func (c *PlaybackCanClient) GetNetwork() string       { return c.network }
func (c *PlaybackCanClient) SetNetwork(n string)      { c.network = n }
func (c *PlaybackCanClient) GetURI() string           { return c.uri }
func (c *PlaybackCanClient) SetURI(uri string)        { c.uri = uri }
func (c *PlaybackCanClient) GetDBCFilePath() *string  { return c.dbcFilePath }
func (c *PlaybackCanClient) SetDBCFilePath(p *string) { c.dbcFilePath = p }
func (c *PlaybackCanClient) IsOpen() bool             { return c.opened }

// GetConnection returns nil — playback reads a file, not a network socket.
func (c *PlaybackCanClient) GetConnection() net.Conn { return nil }

// SetConnection is a no-op — playback reads a file, not a network socket.
func (c *PlaybackCanClient) SetConnection(_ net.Conn) {}

func (c *PlaybackCanClient) GetInterfaceName() string {
	return c.name + c.cfg.CanInterfaceSeparator + c.network + c.cfg.CanInterfaceSeparator + c.uri
}

func (c *PlaybackCanClient) Open() error {
	c.opened = true
	return nil
}

func (c *PlaybackCanClient) Close() error {
	c.opened = false
	return nil
}

func (c *PlaybackCanClient) Discontinue() error {
	c.streaming = false
	return nil
}
