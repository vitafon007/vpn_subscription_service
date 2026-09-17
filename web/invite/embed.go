package inviteweb

import "embed"

// FS — HTML/CSS/JS секретного лендинга.
//
//go:embed templates/*.html static/*
var FS embed.FS
