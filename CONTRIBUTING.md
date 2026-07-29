# Contributing to MiBeeHive

Welcome! We're glad you're interested in contributing to MiBeeHive. This document provides guidelines for contributing to the project.

> MiBeeHive is an **operations tooling supply platform for external servers**: it collects and keeps ops tools up to date and serves them to external servers over standard protocols. It is *not* a local-machine app store like 1Panel — keep this scope in mind when proposing features.

## Development Setup

### Prerequisites
- Go 1.22+ installed
- Git for version control

### Getting Started
```bash
git clone https://github.com/Mi-Bee-Studio/mibeehive.git
cd mibeehive
go mod download
```

### Build and Run
```bash
# Build the main application
go build -o mibeehive ./cmd/mibeehive

# Build the migration tool
go build -o migrate ./cmd/migrate

# Run the application
./mibeehive
```

## Code Style

### Go Code
- Follow standard Go formatting and conventions
- Use proper error wrapping: `fmt.Errorf("context: %w", err)`
- Use structured logging with `log/slog` and key-value pairs
- Keep functions focused and single-purpose
- Use meaningful variable and function names

### Error Handling
```go
// Good
err := db.QueryRow("SELECT * FROM files WHERE id = ?", id).Scan(&file)
if err != nil {
    return nil, fmt.Errorf("db query failed: %w", err)
}

// Bad
err := db.QueryRow("SELECT * FROM files WHERE id = ?", id).Scan(&file)
if err != nil {
    return nil, err
}
```

### Logging
```go
// Good
log.Info("file download started", "file_id", file.ID, "size", file.Size)
log.Debug("retrying download", "attempt", attempt, "max_attempts", maxAttempts)

// Bad
log.Println("Starting file download for file", file.ID)
```

## Frontend Development

### Technology Stack
- **Framework**: Preact + HTM (lightweight React alternative)
- **Styling**: TailwindCSS via CDN
- **Charts**: Chart.js via CDN
- **No npm**: All frontend code is vanilla JavaScript with Preact bridge

### Code Guidelines
- Use CSS variables (`--color-*`) instead of hardcoded colors
- Never use `!important` in CSS
- Use `data-id` attributes for DOM identification during periodic updates
- Prefer targeted DOM manipulation (textContent, classList, appendChild, remove) over `innerHTML`
- Update Chart.js instances in-place: `chart.data = ...; chart.update('none')`
- Preserve progress bar DOM state during data refresh

### Directory Structure
```
web/js/
├── core/         # Framework components
├── layout/       # Shared UI components
└── modules/      # Page-specific modules
```

## Database Migrations

### Migration Rules
- **NEVER modify** `migrations/001_init.sql`
- Always create **new** migration files with sequential numbers
- Use descriptive names: `002_add_user_table.sql`, `003_update_indexes.sql`
- Test migrations in a development environment first

### Migration Process
```sql
-- Example: Add a new column
ALTER TABLE files ADD COLUMN download_url TEXT;

-- Example: Add a new table
CREATE TABLE IF NOT EXISTS downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER,
    status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id)
);
```

## Testing

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/crawler
go test -v ./internal/service

# Run tests with coverage
go test -cover ./...

# Run static analysis
go vet ./...
```

### Test Guidelines
- Write tests for new functionality
- Use table-driven tests for multiple test cases
- Mock external dependencies when appropriate
- Test both success and failure scenarios

## Commit Conventions

### Commit Message Format
```
type(scope): brief description

Detailed explanation (if needed)

# Fixes #123
# Closes #456
```

### Commit Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code formatting
- `refactor`: Code refactoring
- `test`: Test-related changes
- `chore`: Build or auxiliary tool changes

### Examples
```
feat(crawler): add GitHub releases source
fix(file-service): handle network timeouts properly
docs(readme): update installation instructions
refactor(db): optimize query performance
```

## Pull Request Process

### Before Creating a PR
1. Ensure all tests pass: `go test ./...`
2. Run static analysis: `go vet ./...`
3. Update documentation if needed
4. Check for any TODO comments that should be addressed
5. Make sure your changes follow the project's coding style

### PR Template
```markdown
## Changes
- Brief description of changes made
- List any breaking changes
- Include any relevant background information

## Testing
- Describe how the changes were tested
- Include any test cases added

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Code follows style guidelines
- [ ] No breaking changes (unless intentional)
```

### Review Process
1. Submit your PR with a clear description
2. Ensure it passes CI checks
3. Address any review comments promptly
4. Keep the PR focused on a single change
5. Be respectful and collaborative in reviews

## Development Workflow

### Branch Strategy
- `main`: Production-ready code
- `develop`: Integration branch for ongoing development
- Feature branches: Create from `develop`, merge back to `develop`

### Code Review Guidelines
- Be constructive and respectful
- Focus on logic, not personal preferences
- Suggest improvements, don't just criticize
- Test your changes before review
- Respond to comments in a timely manner

## Reporting Issues

### Bug Reports
Use the GitHub issue template and include:
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, etc.)
- Relevant logs or error messages

### Feature Requests
Include:
- Clear description of the requested feature
- Use case and motivation
- Any implementation suggestions
- Potential impact on existing functionality

## Community Guidelines

- Be inclusive and respectful
- Help newcomers when possible
- Focus on technical merit
- Follow the project's code of conduct
- Ask questions if you're unsure

Thank you for contributing to MiBeeHive! 🎉