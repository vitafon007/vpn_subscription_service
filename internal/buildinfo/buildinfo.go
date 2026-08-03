// Package buildinfo хранит метаданные сборки, проставляемые через -ldflags.
package buildinfo

// Значения по умолчанию для локальной сборки без ldflags.
var (
	// Version — семантическая или произвольная метка релиза.
	Version = "dev"
	// Commit — короткий или полный git SHA.
	Commit = "unknown"
	// BuildTime — время сборки (UTC).
	BuildTime = "unknown"
)

// Summary возвращает одну строку для стартового лога.
func Summary() string {
	return "vpn_subscription_service version=" + Version + " commit=" + Commit + " built_at=" + BuildTime
}
