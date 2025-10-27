# Bug Fix: Terminal Resize Causing UI Overflow

**Date:** 2025-10-26
**Severity:** High - UI becomes unusable after resize
**Status:** ✅ Fixed

---

## Problem

When resizing the terminal window, the TUI would lose its top bar (title and tabs) and become unusable. The problem persisted until the application was restarted.

**User Report:**
> "When I resize the terminal window, the tool doesn't display the top bar in most views (any views where the content would be bigger than the vertical portion of the screen). It continues like this until I quit the tool. Everything works fine if I don't resize."

---

## Root Cause

Views were rendering more content than the terminal height, causing the terminal to scroll and push the top bar off-screen.

### The Issue in Detail:

1. On `WindowSizeMsg`, we set `viewHeight = terminalHeight - 10`
2. Each view used this height for its content loop
3. **BUT** each view also added extra lines:
   - Title (2 lines)
   - Subtitle/header (2-3 lines)
   - Scroll indicator (2 lines)
   - Footer/help text (2 lines)

**Result:** A view would output `viewHeight + 8` lines, but the main app also added:
- App title (2 lines)
- Tabs (2 lines)
- View content (viewHeight + 8 lines)
- Skipped info (1-2 lines)
- Help footer (2 lines)

**Total:** Could easily exceed terminal height, causing scrolling.

---

## Solution

Modified each view to account for its own "chrome" (titles, headers, footers) when calculating content height.

### Formula:
```
contentHeight = allocatedHeight - chromeLines
```

Where `chromeLines` is the total lines used by:
- View title
- Subtitles
- Headers
- Scroll indicators
- Footer text

---

## Files Modified

### 1. `ui/views/tree.go`
```go
// BEFORE
end := start + tv.height  // Used full height for content

// AFTER
contentHeight := tv.height - 4  // Reserve for title (2) + scroll indicator (2)
end := start + contentHeight
```

**Lines Changed:** 102-118

---

### 2. `ui/views/toplist.go`
```go
// BEFORE
end := start + tlv.height

// AFTER
contentHeight := tlv.height - 8  // Reserve for title (4) + header (2) + footer (2)
end := start + contentHeight
```

**Lines Changed:** 100-118, 129

---

### 3. `ui/views/errors.go`
```go
// BEFORE
end := start + ev.height

// AFTER
contentHeight := ev.height - 8  // Reserve for title (4) + footer (4)
end := start + contentHeight
```

**Lines Changed:** 65-83, 93

---

### 4. `ui/views/breakdown.go`
```go
// BEFORE
end := start + bv.height

// AFTER
contentHeight := bv.height - 9  // Reserve for title (5) + header (2) + summary (2)
end := start + contentHeight
```

**Lines Changed:** 86-104, 115

---

### 5. `ui/app.go`
**Initial Fix:** Added better documentation and minimum height check

**Follow-up Fix #1 (2025-10-26):** User reported top bar still disappearing after resize. Root cause was excessive vertical spacing between UI elements.

**Changes Made:**
1. Reduced spacing after title from `\n\n` to `\n` (saved 1 line)
2. Updated viewHeight calculation from `msg.Height - 10` to `msg.Height - 6`
3. Updated comments to reflect actual chrome usage

**Follow-up Fix #2 (2025-10-26):** User reported resize still broken. Root causes:
1. Skipped info line was not always accounted for (caused +2 line overflow when shown)
2. Long strings (titles, help text, skipped info) were wrapping to multiple lines

**Changes Made:**
1. Updated viewHeight calculation from `msg.Height - 6` to `msg.Height - 8` (always reserve space for skipped info)
2. Truncated tree view title to max 70 chars (prevents wrapping)
3. Truncated help text to terminal width - 10
4. Truncated skipped info text to terminal width - 10

```go
// WindowSizeMsg handler - final calculation
// Reserve space for:
// - Title (1 line) + newline (1 line)
// - Tabs (1 line) + newline (1 line)
// - Newline before help (1 line) + Help footer (1 line)
// - Newline before skipped (1 line) + Skipped info (1 line)
// Total app chrome: 8 lines (always, even if skipped not shown)
viewHeight := msg.Height - 8
if viewHeight < 5 {
    viewHeight = 5 // Minimum height
}

// Tree view - truncate title to prevent wrapping
fullTitle := title + sortIndicator + zoomIndicator
if len(fullTitle) > 70 {
    fullTitle = fullTitle[:67] + "..."
}

// Help text - truncate to terminal width
if len(helpText) > maxWidth {
    helpText = helpText[:maxWidth-3] + "..."
}
```

**Lines Changed:**
- `ui/app.go`: 111-125 (WindowSizeMsg), 486-495 (renderHelp), 458-467 (renderSkippedInfo)
- `ui/views/tree.go`: 154-172 (title truncation)

---

## Testing

### Before Fix:
```bash
# Start app
./spaceforce -path ~

# Resize terminal window (make it smaller)
# Result: Top bar disappears, UI broken
```

### After Fix:
```bash
# Start app
./spaceforce -path ~

# Resize terminal window (any size)
# Result: UI adjusts correctly, top bar always visible
```

### Test Cases:

1. ✅ **Small terminal (24 rows)**
   - Views render with minimum content
   - Top bar always visible
   - Navigation works

2. ✅ **Large terminal (100 rows)**
   - Views show more content
   - Top bar visible
   - Navigation works

3. ✅ **Dynamic resize (small → large → small)**
   - Views adjust smoothly
   - No scrolling issues
   - Top bar always visible

4. ✅ **All views tested**
   - Tree View
   - Top Items
   - Breakdown
   - Timeline
   - Errors

---

## Additional Safety Improvements

While fixing the resize bug, also improved safety protection for user directories:

### Protected Directories Added:
- `/Users/<username>` - Home directory itself
- `/Users/<username>/Library` - Entire Library folder
- `~/.ssh` - SSH keys
- `~/.gnupg` - GPG keys
- `~/.aws` - AWS credentials
- `~/.config` - Config files
- `~/.kube` - Kubernetes config
- `~/.docker` - Docker config

**File Modified:** `safety/protector.go`

**Reasoning:**
User correctly noted that the home directory and Library folder should not be considered "safe to delete" as they contain critical system and application data.

---

## Technical Notes

### Why Not Use lipgloss.Height()?

Lipgloss's `Height()` function measures rendered content but:
1. Requires rendering first (performance cost)
2. Doesn't help with pre-calculation
3. Views need to know limits before rendering

**Better approach:** Track chrome lines as constants/calculations.

### Content Height Pattern

All views now follow this pattern:

```go
func (v *View) View() string {
    // Calculate chrome overhead
    chromeLines := titleLines + headerLines + footerLines
    contentHeight := v.height - chromeLines
    if contentHeight < 1 {
        contentHeight = 1  // Minimum
    }

    // Use contentHeight for viewport calculations
    start := v.selectedIndex - contentHeight/2
    end := start + contentHeight

    // Render items (max contentHeight lines)
    for i := start; i < end; i++ {
        // ...
    }
}
```

---

## Regression Testing

Run these tests to ensure fix doesn't break existing functionality:

```bash
# 1. Normal usage (no resize)
./spaceforce -path ~/Documents
# Navigate around, switch views - should work as before

# 2. Resize testing
./spaceforce -path ~/Documents
# Resize terminal multiple times
# All views should remain functional

# 3. Minimum size
./spaceforce -path ~/Documents
# Resize to very small (24x80)
# Should still be usable (though cramped)

# 4. Maximum size
./spaceforce -path ~/Documents
# Resize to very large (100x200)
# Should show more content, no scrolling issues

# 5. All views
# Test each view (1-5) with various sizes
# All should render correctly
```

---

## Performance Impact

**Minimal:** The fix adds simple arithmetic (`height - chromeLines`) which is negligible.

**No impact on:**
- Scanning speed
- Navigation responsiveness
- View switching
- File operations

---

## Future Improvements

1. **Dynamic chrome calculation**: Instead of hardcoding chrome line counts, calculate them from actual rendered chrome
2. **Responsive chrome**: Hide less important elements (subtitles, scroll indicators) on very small terminals
3. **Better minimum handling**: Show helpful message if terminal is too small (<20 rows)

---

## Summary

| Aspect | Before | After Initial Fix | After Follow-up #1 | After Follow-up #2 |
|--------|--------|----------|----------|----------|
| Resize handling | ❌ Broken | ⚠️ Partial | ⚠️ Still broken | ✅ Works |
| Top bar visibility | ❌ Lost after resize | ⚠️ Still lost | ⚠️ Still lost | ✅ Always visible |
| View overflow | ❌ Scrolling issues | ⚠️ Still scrolling | ⚠️ Scrolling with skipped info | ✅ Properly constrained |
| Text wrapping | ❌ Long strings wrap | ❌ Still wrapping | ❌ Still wrapping | ✅ Truncated |
| Vertical spacing | ❌ Excessive | ⚠️ Improved | ✅ Compact | ✅ Compact |
| Chrome calculation | ❌ Wrong (10 lines) | ⚠️ Partial (6 lines) | ⚠️ Missing skipped (6 lines) | ✅ Correct (8 lines) |
| User experience | ❌ Must restart | ⚠️ Must restart | ⚠️ Must restart | ✅ Seamless |
| Safety protection | ⚠️  Limited | ✅ Enhanced | ✅ Enhanced | ✅ Enhanced |

**Result:** Terminal resizing now works correctly with:
- Accurate chrome calculation (8 lines: always reserve space for skipped info)
- Text truncation prevents wrapping (titles, help, skipped info)
- All views properly constrained to terminal height
- Top bar always remains visible

---

**Fixed By:** Height accounting in all view renderers
**Tested On:** macOS (Darwin 25.0.0)
**Related Issues:** None
**Document Version:** 1.0
