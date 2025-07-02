# Contributing to gograph

Thank you for your interest in contributing to gograph! This document provides guidelines and information for contributors.

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contribution Workflow](#contribution-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)
- [Review Process](#review-process)

## 🤝 Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow. Please be respectful and constructive in all interactions.

### Our Standards

- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Gracefully accept constructive criticism
- Focus on what is best for the community
- Show empathy towards other community members

## 🚀 Getting Started

### Types of Contributions

We welcome various types of contributions:

- **Bug Reports**: Help us identify and fix issues
- **Feature Requests**: Suggest new functionality
- **Code Contributions**: Bug fixes, new features, improvements
- **Documentation**: Improve docs, examples, tutorials
- **Testing**: Add test cases, improve coverage
- **Performance**: Optimize existing functionality

### Before You Start

1. **Check existing issues**: Look for existing issues or discussions
2. **Open an issue**: For new features or significant changes, open an issue first
3. **Discuss**: Engage with maintainers and community about your idea
4. **Plan**: Understand the scope and approach before coding

## 🛠 Development Setup

### Prerequisites

- Go 1.24 or higher
- Neo4j 5.x
- Make
- Docker (for integration tests)
- Git

### Setup Instructions

1. **Fork the repository**:

   ```bash
   # Fork on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/gograph.git
   cd gograph
   ```

2. **Add upstream remote**:

   ```bash
   git remote add upstream https://github.com/compozy/gograph.git
   ```

3. **Install dependencies**:

   ```bash
   make deps
   ```

4. **Start development environment**:

   ```bash
   make dev
   ```

5. **Verify setup**:
   ```bash
   make test
   make lint
   make build
   ```

### Development Environment

The project uses:

- **Neo4j**: Graph database for storing code structure
- **Docker**: For running Neo4j and integration tests
- **Make**: Build automation and task runner
- **golangci-lint**: Code linting and formatting
- **testify**: Testing framework

## 🔄 Contribution Workflow

### 1. Create a Branch

```bash
# Update your fork
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

### 2. Make Changes

- Follow the [coding standards](#coding-standards)
- Add tests for new functionality
- Update documentation as needed
- Ensure all tests pass

### 3. Commit Changes

Use conventional commit messages:

```bash
# Format: type(scope): description
git commit -m "feat(parser): add support for generic type parsing"
git commit -m "fix(graph): resolve circular dependency detection"
git commit -m "docs: update MCP integration guide"
git commit -m "test: add integration tests for analyzer"
```

**Commit Types:**

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

### 4. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## 📝 Coding Standards

### Go Standards

Follow the project's coding standards defined in `.cursor/rules/`:

- **gofmt**: All code must be formatted with `gofmt`
- **golangci-lint**: Must pass all linter checks
- **Function Length**: Keep functions under 30 lines for business logic
- **Line Length**: Maximum 120 characters per line
- **Error Handling**: Always handle errors explicitly

### Architecture Principles

- **Clean Architecture**: Dependencies point inward toward the domain
- **Domain-Driven Design**: Clear domain boundaries
- **Interface Segregation**: Small, focused interfaces
- **Dependency Injection**: Constructor-based injection
- **Single Responsibility**: Each component has one reason to change

### Code Examples

```go
// ✅ Good: Clear function with single responsibility
func (s *ParserService) ParseFile(ctx context.Context, filePath string) (*ast.File, error) {
    if filePath == "" {
        return nil, core.NewError(
            errors.New("file path is required"),
            "INVALID_INPUT",
            map[string]any{"path": filePath},
        )
    }

    content, err := s.fileReader.ReadFile(ctx, filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
    }

    return s.parseContent(ctx, content)
}

// ✅ Good: Constructor with dependency injection
func NewParserService(fileReader FileReader, config *ParserConfig) *ParserService {
    if config == nil {
        config = DefaultParserConfig()
    }
    return &ParserService{
        fileReader: fileReader,
        config:     config,
    }
}
```

### Naming Conventions

- **Packages**: Short, lowercase, single word
- **Types**: PascalCase, descriptive names
- **Functions**: PascalCase for exported, camelCase for private
- **Variables**: camelCase, intention-revealing names
- **Constants**: PascalCase or UPPER_CASE for package-level

## 🧪 Testing Guidelines

### Test Structure

All tests must follow the established pattern:

```go
func TestServiceName(t *testing.T) {
    t.Run("Should describe expected behavior", func(t *testing.T) {
        // Arrange
        service := setupTestService()
        input := "test input"

        // Act
        result, err := service.Method(context.Background(), input)

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, expectedResult, result)
    })
}
```

### Test Types

1. **Unit Tests**: Fast, isolated tests for business logic
2. **Integration Tests**: Tests with real Neo4j database
3. **E2E Tests**: End-to-end CLI testing

### Test Requirements

- Use `testify/assert` and `testify/mock`
- Achieve 80%+ test coverage for new code
- Test both happy paths and error cases
- Use descriptive test names with "Should" prefix
- Clean up resources in tests

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run integration tests
make test-integration

# Run specific test
go test -run TestParserService ./engine/parser/
```

## 📚 Documentation

### Documentation Requirements

- Update README.md for new features
- Add inline comments for complex logic
- Update API documentation
- Include examples in documentation
- Update MCP integration guide if applicable

### Documentation Style

- Use clear, concise language
- Include code examples
- Provide context and rationale
- Use proper markdown formatting
- Link to related documentation

## 📤 Submitting Changes

### Pull Request Guidelines

1. **Title**: Use descriptive title following conventional commits
2. **Description**: Explain what, why, and how
3. **Testing**: Describe how you tested the changes
4. **Breaking Changes**: Clearly document any breaking changes
5. **Related Issues**: Link to related issues

### PR Template

```markdown
## Description

Brief description of the changes.

## Type of Change

- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed

## Checklist

- [ ] Code follows project standards
- [ ] Tests pass locally
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
```

### Pre-submission Checklist

Before submitting your PR:

- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Code builds successfully (`make build`)
- [ ] Documentation updated
- [ ] Conventional commit messages used
- [ ] PR description filled out completely

## 🔍 Review Process

### What to Expect

1. **Automated Checks**: CI runs tests, linting, and security scans
2. **Code Review**: Maintainers review code quality and design
3. **Feedback**: Reviewers may request changes or ask questions
4. **Iteration**: Make requested changes and push updates
5. **Approval**: Once approved, changes will be merged

### Review Criteria

Reviewers check for:

- **Correctness**: Does the code work as intended?
- **Quality**: Is the code readable and maintainable?
- **Standards**: Does it follow project conventions?
- **Testing**: Are there adequate tests?
- **Documentation**: Is documentation updated?
- **Performance**: Are there any performance concerns?
- **Security**: Are there any security implications?

### Addressing Feedback

- Respond to all comments
- Make requested changes promptly
- Ask questions if feedback is unclear
- Be open to suggestions and improvements
- Update the PR description if scope changes

## 🏷️ Release Process

### Versioning

We use [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Schedule

- **Patch releases**: As needed for critical fixes
- **Minor releases**: Monthly or when significant features are ready
- **Major releases**: When breaking changes are necessary

## 🆘 Getting Help

### Communication Channels

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Pull Requests**: For code review and collaboration

### Asking Questions

When asking for help:

1. **Search first**: Check existing issues and documentation
2. **Be specific**: Provide context and details
3. **Include examples**: Show what you've tried
4. **Be patient**: Maintainers volunteer their time

## 📋 Issue Templates

### Bug Report

```markdown
**Describe the bug**
A clear description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior.

**Expected behavior**
What you expected to happen.

**Environment**

- OS: [e.g. macOS, Linux]
- Go version: [e.g. 1.24]
- gograph version: [e.g. v1.0.0]

**Additional context**
Any other context about the problem.
```

### Feature Request

```markdown
**Is your feature request related to a problem?**
A clear description of what the problem is.

**Describe the solution you'd like**
A clear description of what you want to happen.

**Describe alternatives you've considered**
Any alternative solutions or features you've considered.

**Additional context**
Any other context about the feature request.
```

## 🙏 Recognition

Contributors are recognized in:

- Release notes
- Contributors section in README
- GitHub contributors graph
- Special thanks in major releases

Thank you for contributing to gograph! Your efforts help make this project better for everyone.
