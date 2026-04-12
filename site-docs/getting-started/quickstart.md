# Quick Start

## 1. Install

```bash
go install mgtt/cmd/mgtt@latest
```

## 2. Write a model

```bash
mgtt init
# Edit system.model.yaml with your components
```

## 3. Validate

```bash
mgtt model validate
```

## 4. Write scenarios

```yaml
# scenarios/db-down.yaml
name: database unavailable
inject:
  db:
    available: false
expect:
  root_cause: db
```

## 5. Simulate

```bash
mgtt simulate --all
```

## 6. Troubleshoot

```bash
mgtt incident start
mgtt plan
# Press Y at each probe
mgtt incident end
```
