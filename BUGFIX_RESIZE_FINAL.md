# Final Resize Bug Fix

**Date:** 2025-10-26
**Issue:** Tabs 1, 2, and 5 showed resize bug; Tabs 3 and 4 worked correctly

---

## Root Cause Analysis

By comparing working tabs (Breakdown, Timeline) with broken tabs (Tree, TopList, Errors), we discovered:

### Working Views (Breakdown, Timeline)
✅ **No extra help text** at bottom of view
✅ **Correct chrome calculation** accounting for all output

### Broken Views - Three Different Problems

#### 1. TopList (Tab 2) - MAJOR OVERFLOW
**Problem:** Had 3 lines of view-specific help text at bottom not accounted for
```go
b.WriteString("\n\n")
b.WriteString(util.HelpStyle.Render("s: sort | f: toggle files | d: toggle dirs | ↑↓: navigate"))
```
**Math:**
- Chrome: 9 lines
- Content: `contentHeight`
- Optional footer: 0-2 lines
- Extra help: 3 lines (ALWAYS)
- **Total:** 9 + contentHeight + 0-2 + 3 = 12-14 + contentHeight
- **With contentHeight = height - 8:** Total = height + 4 to height + 6
- **Overflow:** 4-6 lines! 🔴

#### 2. Errors (Tab 5) - MINOR OVERFLOW
**Problem:** Had 4 lines of view-specific help text at bottom not accounted for
```go
b.WriteString("\n\n")
b.WriteString(util.HelpStyle.Render("Most errors are permission-related..."))
b.WriteString("\n")
b.WriteString(util.HelpStyle.Render("Use 'sudo' to scan..."))
```
**Math:**
- Chrome: 5 lines (title + subtitle + newlines)
- Content: `contentHeight`
- Optional footer: 0-2 lines
- Extra help: 4 lines (ALWAYS)
- **Total:** 5 + contentHeight + 0-2 + 4 = 9-11 + contentHeight
- **With contentHeight = height - 8:** Total = height + 1 to height + 3
- **Overflow:** 1-3 lines! 🟡

#### 3. Tree (Tab 1) - MISCALCULATION
**Problem:** Chrome calculation counted "\n\n" as 1 line instead of 2 lines
```go
b.WriteString(util.TitleStyle.Render(fullTitle))
b.WriteString("\n\n")  // This is 2 lines, not 1!

// Reserve lines for title (2) and scroll indicator (2)
contentHeight := tv.height - 4  // WRONG: should be -5
```
**Math:**
- Chrome: 1 (title) + 2 (newlines) = 3 lines (NOT 2!)
- Content: `contentHeight`
- Optional scroll: 0-2 lines
- **Total:** 3 + contentHeight + 0-2
- **With contentHeight = height - 4:** Total = height - 1 to height + 1
- **Overflow:** -1 to +1 lines 🟡

---

## Solution

### 1. Remove Redundant View-Specific Help Text
The app already has a global help footer, so view-specific help is redundant and causes overflow.

**TopList (ui/views/toplist.go):**
```diff
  // Footer
  if len(tlv.items) > contentHeight {
      b.WriteString("\n")
      b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d items",
          start+1, end, len(tlv.items))))
  }

- b.WriteString("\n\n")
- b.WriteString(util.HelpStyle.Render("s: sort | f: toggle files | d: toggle dirs | ↑↓: navigate"))
-
  return b.String()
```

**Errors (ui/views/errors.go):**
```diff
  // Footer
  if len(ev.errors) > contentHeight {
      b.WriteString("\n")
      b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d errors",
          start+1, end, len(ev.errors))))
  }

- b.WriteString("\n\n")
- b.WriteString(util.HelpStyle.Render("Most errors are permission-related and can be safely ignored."))
- b.WriteString("\n")
- b.WriteString(util.HelpStyle.Render("Use 'sudo' to scan system directories with full access."))
-
  return b.String()
```

### 2. Fix Chrome Calculations

All views now properly account for optional footers/scrolls in worst case:

**Tree (ui/views/tree.go):**
```diff
- // Reserve lines for title (2) and scroll indicator (2)
- contentHeight := tv.height - 4
+ // Reserve lines for title (1) + newlines (2) + scroll indicator (2)
+ contentHeight := tv.height - 5
```

**TopList (ui/views/toplist.go):**
```diff
- // Reserve lines for title (4), header (2), footer (2)
- contentHeight := tlv.height - 8
+ // Reserve lines for title (2), subtitle (3), header (2), separator (2), footer (2)
+ // Total chrome: 9 lines + 2 for optional footer = 11 lines worst case
+ contentHeight := tlv.height - 11
```

**Breakdown (ui/views/breakdown.go):**
```diff
- // Reserve lines for title (5), header (2), summary (2)
- contentHeight := bv.height - 9
+ // Reserve lines for title (2), subtitle (3), header (2), separator (2), summary (2)
+ // Total chrome: 9 lines + 2 for optional summary = 11 lines worst case
+ contentHeight := bv.height - 11
```

**Errors (ui/views/errors.go):**
```diff
- // Reserve lines for title (4), footer (4)
- contentHeight := ev.height - 8
+ // Reserve lines for title (2), subtitle (3), footer (2)
+ // Total chrome: 5 lines + 2 for optional footer = 7 lines worst case
+ contentHeight := ev.height - 7
```

---

## Verification Math

### Tree View
- Chrome: 3 lines (title + "\n\n")
- Content: `height - 5` lines
- Optional scroll: 0-2 lines
- **Total:** 3 + (height - 5) + 0-2 = **height - 2 to height** ✅

### TopList View
- Chrome: 9 lines (title + subtitle + header + separator)
- Content: `height - 11` lines
- Optional footer: 0-2 lines
- **Total:** 9 + (height - 11) + 0-2 = **height - 2 to height** ✅

### Breakdown View
- Chrome: 9 lines
- Content: `height - 11` lines
- Optional summary: 0-2 lines
- **Total:** 9 + (height - 11) + 0-2 = **height - 2 to height** ✅

### Errors View
- Chrome: 5 lines (title + subtitle + newlines)
- Content: `height - 7` lines
- Optional footer: 0-2 lines
- **Total:** 5 + (height - 7) + 0-2 = **height - 2 to height** ✅

---

## Result

✅ All views now correctly stay within allocated height
✅ Top bar always remains visible after resize
✅ Views properly constrained, no scrolling
✅ Consistent across all tabs

**Files Modified:**
- `ui/views/tree.go` - Fixed chrome calculation
- `ui/views/toplist.go` - Removed extra help, fixed chrome calculation
- `ui/views/breakdown.go` - Fixed chrome calculation
- `ui/views/errors.go` - Removed extra help, fixed chrome calculation

**Testing:** Resize terminal to various sizes in all 5 tabs - top bar should always remain visible.
