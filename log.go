package main

import (
	"io"
	"os"

	"github.com/elastos/Elastos.ELA.SideChain/blockchain"
	sm "github.com/elastos/Elastos.ELA.SideChain/mempool"
	"github.com/elastos/Elastos.ELA.SideChain/netsync"
	"github.com/elastos/Elastos.ELA.SideChain/peer"
	"github.com/elastos/Elastos.ELA.SideChain/pow"
	"github.com/elastos/Elastos.ELA.SideChain/server"
	ser "github.com/elastos/Elastos.ELA.SideChain/service"
	"github.com/elastos/Elastos.ELA.SideChain/spv"

	"github.com/elastos/Elastos.ELA.Utility/elalog"
	"github.com/elastos/Elastos.ELA.Utility/http/jsonrpc"
	"github.com/elastos/Elastos.ELA.Utility/http/restful"
	"github.com/elastos/Elastos.ELA.Utility/p2p/addrmgr"
	"github.com/elastos/Elastos.ELA.Utility/p2p/connmgr"

	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/avm"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/store"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/smartcontract/service"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/service/websocket"
)

const (
	defaultMaxPerLogFileSize int64 = elalog.MBSize * 20
	defaultMaxLogsFolderSize int64 = elalog.GBSize * 2
)

// configFileWriter returns the configured parameters for log file writer.
func configFileWriter() (string, int64, int64) {
	maxPerLogFileSize := defaultMaxPerLogFileSize
	maxLogsFolderSize := defaultMaxLogsFolderSize
	if cfg.MaxPerLogFileSize > 0 {
		maxPerLogFileSize = cfg.MaxPerLogFileSize * elalog.MBSize
	}
	if cfg.MaxLogsFolderSize > 0 {
		maxLogsFolderSize = cfg.MaxLogsFolderSize * elalog.MBSize
	}
	return defaultLogDir, maxPerLogFileSize, maxLogsFolderSize
}

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var (
	fileWriter = elalog.NewFileWriter(configFileWriter())
	logWriter  = io.MultiWriter(os.Stdout, fileWriter)
	backend    = elalog.NewBackend(logWriter, elalog.Llongfile)
	level, _ = elalog.LevelFromString(cfg.LogLevel)

	admrlog = backend.Logger("ADMR", elalog.LevelOff)
	cmgrlog = backend.Logger("CMGR", elalog.LevelOff)
	bcdblog = backend.Logger("BCDB", level)
	txmplog = backend.Logger("TXMP", level)
	synclog = backend.Logger("SYNC", level)
	peerlog = backend.Logger("PEER", level)
	minrlog = backend.Logger("MINR", level)
	spvslog = backend.Logger("SPVS", level)
	srvrlog = backend.Logger("SRVR", level)
	httplog = backend.Logger("HTTP", level)
	rpcslog = backend.Logger("RPCS", level)
	restlog = backend.Logger("REST", level)
	eladlog = backend.Logger("ELAD", level)

	sockLog = backend.Logger("SOCKET", level)
	avmlog  = backend.Logger("AVM", level)
)

func setLogLevel(level elalog.Level) {
	bcdblog.SetLevel(level)
	txmplog.SetLevel(level)
	synclog.SetLevel(level)
	peerlog.SetLevel(level)
	minrlog.SetLevel(level)
	spvslog.SetLevel(level)
	srvrlog.SetLevel(level)
	httplog.SetLevel(level)
	rpcslog.SetLevel(level)
	restlog.SetLevel(level)
	eladlog.SetLevel(level)
	sockLog.SetLevel(level)
	avmlog.SetLevel(level)
}

// The default amount of logging is none.
func init() {
	addrmgr.UseLogger(admrlog)
	connmgr.UseLogger(cmgrlog)
	blockchain.UseLogger(bcdblog)
	sm.UseLogger(txmplog)
	netsync.UseLogger(synclog)
	peer.UseLogger(peerlog)
	server.UseLogger(srvrlog)
	pow.UseLogger(minrlog)
	spv.UseLogger(spvslog)
	ser.UseLogger(httplog)
	jsonrpc.UseLogger(rpcslog)
	restful.UseLogger(restlog)

	avm.UseLogger(avmlog)
	store.UseLogger(avmlog)
	service.UseLogger(avmlog)
	websocket.UseLogger(sockLog)
}
