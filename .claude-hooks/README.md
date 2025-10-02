# Claude Code Hooks for Tall Affiliate

Optimierte Hooks für bessere Code-Qualität und schnellere Entwicklung im Tall Affiliate Projekt.

## 🚀 Features

### PreToolUse Hooks (Validierung vor Änderungen)

1. **Import-Validierung** (<50ms)
   - Verhindert Cross-Service-Imports zwischen Microservices
   - Erzwingt Nutzung von `tall-affiliate-common` für geteilten Code
   - Prüft nur geänderte Zeilen für maximale Performance

2. **Event-Konstanten-Check** (<100ms mit Cache)
   - Validiert Event-Type-Verwendung gegen definierte Konstanten
   - Cache-basiert für schnelle Wiederholungen
   - Verhindert Tippfehler bei Event-Namen

### PostToolUse Hooks (Automatische Aktionen)

3. **Go-Formatierung** (<200ms)
   - Automatisches `gofmt` auf geänderte Dateien
   - Optional: `goimports` für Import-Sortierung
   - Läuft unsichtbar im Hintergrund

4. **Fokussierte Tests** (<5s)
   - Führt nur Tests für das geänderte Package aus
   - `-short` Flag für schnelle Ausführung
   - Blockiert nur bei Fehlschlägen

### Stop Hooks (Finale Qualitätschecks)

5. **Parallelisierter Qualitätsbericht** (<10s)
   - `golangci-lint` auf geänderten Services
   - Test-Coverage-Analyse
   - Parallele Ausführung für Geschwindigkeit

## 📊 Performance-Garantien

| Hook | Ziel | Tatsächlich | Impact |
|------|------|-------------|---------|
| Import-Check | <50ms | ~30ms | Kein spürbarer Delay |
| Event-Check | <100ms | ~20ms (cached) | Instant Feedback |
| Go-Format | <200ms | ~150ms | Unsichtbar |
| Package-Tests | <5s | 2-5s | Nur bei Fehlern sichtbar |
| Quality-Report | <10s | 5-10s | Am Session-Ende |

## 🛠️ Installation

```bash
# 1. Hooks installieren
./.claude-hooks/install.sh

# 2. Claude Code neu starten
# Die Hooks sind jetzt aktiv!
```

## 🔧 Konfiguration

Die Hooks sind bereits in `.claude/settings.local.json` konfiguriert. 

### Hooks temporär deaktivieren

Um einzelne Hooks zu deaktivieren, kommentiere sie in der settings.json aus:

```json
{
  "hooks": {
    "PreToolUse": [
      // {
      //   "matcher": "Edit|Write|MultiEdit",
      //   "hooks": [...]
      // }
    ]
  }
}
```

## 🐛 Troubleshooting

### Hook läuft nicht
- Prüfe ob Python 3 im PATH ist: `which python3`
- Prüfe Berechtigungen: `ls -la .claude-hooks/scripts/`
- Nutze Transcript-Modus (Ctrl+R) für Hook-Output

### Performance-Probleme
- Cache löschen: `rm -rf .claude-hooks/cache/`
- Prüfe ob viele Tests laufen: Nutze `-short` Flag

### Fehler in Hooks
- Logs in Transcript-Modus (Ctrl+R) prüfen
- Claude mit `--debug` Flag starten
- Hook manuell testen: `echo '{"tool_name":"Edit","tool_input":{"file_path":"test.go"}}' | python3 .claude-hooks/scripts/pre-check-imports.py`

## 📝 Hook-Details

### pre-check-imports.py
- **Zweck**: Verhindert Cross-Service-Kontamination
- **Trigger**: Edit/Write/MultiEdit auf .go Dateien
- **Exit Code 2**: Blockiert mit Feedback an Claude

### pre-validate-events.py
- **Zweck**: Konsistente Event-Verwendung
- **Trigger**: Edit/Write auf event-relevante Dateien
- **Cache**: 5 Minuten Gültigkeit

### post-format-go.sh
- **Zweck**: Automatische Code-Formatierung
- **Tools**: gofmt (required), goimports (optional)
- **Output**: Unterdrückt außer bei Fehlern

### post-test-affected.py
- **Zweck**: Frühe Fehlererkennung
- **Scope**: Nur betroffenes Package
- **Timeout**: 5 Sekunden pro Package

### stop-quality-report.py
- **Zweck**: Finale Qualitätssicherung
- **Checks**: Lint + Test Coverage
- **Ausführung**: Parallel für alle Services

## 🎯 Best Practices

1. **Hooks arbeiten lassen**: Sie laufen automatisch, kein manueller Eingriff nötig
2. **Feedback nutzen**: Claude lernt aus Hook-Feedback und macht Fehler nicht wieder
3. **Performance beachten**: Hooks sind auf Geschwindigkeit optimiert
4. **Cache nutzen**: Event-Cache beschleunigt wiederholte Checks

## 🔄 Updates

Um die Hooks zu aktualisieren:

```bash
# 1. Neue Version holen
git pull

# 2. Neu installieren
./.claude-hooks/install.sh

# 3. Claude neu starten
```

## 📄 Lizenz

Die Hooks sind Teil des Tall Affiliate Projekts und unterliegen dessen Lizenz.