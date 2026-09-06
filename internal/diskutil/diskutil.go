// Package diskutil reports filesystem usage for a path while isolating the
// platform-specific statfs syscalls behind per-OS files, so the rest of the
// codebase stays portable (go vet/test must also type-check on Windows dev
// hosts even though the production target is Linux).
package diskutil
