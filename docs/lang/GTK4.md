---
title: GTK4 Conventions
weight: 67
---

<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# GTK4 and PyGObject Development Guidelines

This document details development guidelines, architectural patterns, and resolved pitfalls for PyGObject and GTK4 UI programming in Sprite View.

---

## 1. Application Uniqueness

* **Standard**: Always use `Gio.ApplicationFlags.NON_UNIQUE` application flags.
* **Context**: When Sprite View is spawned from Nautilus or a command-line script, a unique application instance flag would cause subsequent spawns to silently send files to the background daemon instead of opening a new CLI window. Using non-unique flags ensures each invocation gets its own independent process and window context.

---

## 2. Event Gesture Consolidation

* **Standard**: Avoid placing both `Gtk.GestureClick` and `Gtk.GestureDrag` on the same drawing canvas or widget. Consolidate click and drag event handling inside a single `Gtk.GestureDrag` controller.
* **Context**: GTK4 coordinates mouse press and release across gesture controllers. A click triggers a brief press state, firing `drag-begin` immediately followed by `drag-end` (with zero or near-zero displacement). If click reset logic runs alongside drag tracking, the release event clears the active crop box or overlay state, resulting in flickers where the drawing canvas appears and instantly vanishes.
* **Implementation Pattern**:
  ```python
  # Setup the single drag gesture
  drag_gesture = Gtk.GestureDrag.new()
  drag_gesture.connect("drag-begin", self._on_drag_begin)
  drag_gesture.connect("drag-update", self._on_drag_update)
  drag_gesture.connect("drag-end", self._on_drag_end)
  self.widget.add_controller(drag_gesture)
  ```
  Distinguish simple clicks inside `_on_drag_end` by checking offset displacement:
  ```python
  def _on_drag_end(self, gesture, offset_x, offset_y):
      is_click = abs(offset_x) < 5.0 and abs(offset_y) < 5.0
      if is_click:
          # Handle simple click click-action
          self._on_click_detected()
      else:
          # Keep the dragged selection box
          pass
  ```

---

## 3. Cairo Overlay Drawing Areas

* **Standard**:
  1. Set alignment properties of drawing area overlays to `FILL`.
  2. Use size parameters passed directly to the draw callback instead of calling `widget.get_width()`.
* **Context**: `Gtk.Overlay` child widgets may not expand to cover the parent layout if alignments are unspecified. Additionally, calling `get_width()` / `get_height()` on widgets during layout stages might return incomplete layout info (`0` or `1`).
* **Implementation Pattern**:
  ```python
  self.crop_overlay = Gtk.DrawingArea()
  self.crop_overlay.set_halign(Gtk.Align.FILL)
  self.crop_overlay.set_valign(Gtk.Align.FILL)
  self.crop_overlay.set_draw_func(self._on_crop_overlay_draw)
  ```
  Use `w` and `h` arguments in the draw function:
  ```python
  def _on_crop_overlay_draw(self, area, cr, w, h):
      # w and h represent the actual allocated screen pixels
      bounds = self._get_image_bounds(w, h)
  ```

---

## 4. Integration Smoke Testing

* **Standard**: Always run integration smoke tests (`test_integration.py`) to verify window presentation behavior.
* **Context**: Import-time and startup-time AttributeErrors or Gtk layout crashes will not be caught by static code analysis or mocks. Running the compiled executable inside a virtual framebuffer ensures the application launches successfully.
