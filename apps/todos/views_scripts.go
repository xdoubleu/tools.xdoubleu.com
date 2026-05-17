//nolint:lll // JavaScript string literals inside Go cannot be line-wrapped
package todos

import (
	"fmt"
	"strings"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

// buildListPageInitScript builds the JS variable declarations that depend on
// Go data. The result is injected via templ.Raw to avoid templ trying to
// parse Go expressions inside a <script> block.
func buildListPageInitScript(d ListPageData) string {
	var sb strings.Builder
	sb.WriteString("var PRESET_LABELS=[")
	for i, p := range d.Presets.Labels {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('"')
		sb.WriteString(strings.ReplaceAll(p.Value, `"`, `\"`))
		sb.WriteByte('"')
	}
	sb.WriteString("];")

	sb.WriteString("var PRESET_SECTIONS=[")
	for i, sec := range d.Sections {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{id:"%s",name:"%s"}`,
			sec.ID.String(),
			strings.ReplaceAll(sec.Name, `"`, `\"`),
		)
	}
	sb.WriteString("];")

	sb.WriteString("var URL_PATTERNS=[")
	for i, p := range d.Patterns {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{prefix:"%s",platform:"%s",shortcut:"%s"}`,
			strings.ReplaceAll(p.URLPrefix, `"`, `\"`),
			strings.ReplaceAll(p.PlatformName, `"`, `\"`),
			strings.ReplaceAll(p.Shortcut, `"`, `\"`),
		)
	}
	sb.WriteString("];")

	var wsID string
	if d.UserSettings != nil && d.UserSettings.ActiveWorkspaceID != nil {
		wsID = d.UserSettings.ActiveWorkspaceID.String()
	}
	fmt.Fprintf(&sb, `var ACTIVE_WORKSPACE_ID="%s";`, wsID)

	return sb.String()
}

// listPageStaticJS is the static body of the list page JS (no Go interpolation).
// It begins after the four dynamic variable declarations and ends before the
// closing }()); of the outer IIFE.
const listPageStaticJS = `
  /* ── Label autocomplete dropdown ──────────────────────────────────────── */
  var labelAcDrop = null; // Lazy singleton dropdown element
  var labelAcOwner = null; // Which input currently owns the dropdown

  function initLabelAc(inputEl, presets) {
    inputEl.addEventListener('input', function () {
      var val = inputEl.value;
      var lastCommaIdx = val.lastIndexOf(',');
      var segment = lastCommaIdx >= 0
        ? val.slice(lastCommaIdx + 1).trim()
        : val.trim();

      if (!segment) {
        if (labelAcDrop) { labelAcDrop.style.display = 'none'; }
        return;
      }

      var matches = presets.filter(function (p) {
        return p.toLowerCase().startsWith(segment.toLowerCase());
      });

      if (!labelAcDrop) {
        labelAcDrop = document.createElement('ul');
        labelAcDrop.className = 'list-group shadow-sm';
        labelAcDrop.style.cssText = 'position:fixed;z-index:9999;display:none;max-height:200px;overflow-y:auto';
        document.body.appendChild(labelAcDrop);
      }

      labelAcOwner = inputEl;

      labelAcDrop.innerHTML = '';
      matches.forEach(function (val) {
        var li = document.createElement('li');
        li.className = 'list-group-item';
        li.style.cssText = 'cursor:pointer;padding:.35rem .75rem;font-size:.875rem';
        li.textContent = val;
        li.addEventListener('mousedown', function (e) {
          e.preventDefault();
          var currentVal = inputEl.value;
          var lastIdx = currentVal.lastIndexOf(',');
          if (lastIdx >= 0) {
            inputEl.value = currentVal.slice(0, lastIdx + 1) + ' ' + val;
          } else {
            inputEl.value = val;
          }
          if (labelAcDrop) { labelAcDrop.style.display = 'none'; }
        });
        labelAcDrop.appendChild(li);
      });

      if (matches.length) {
        var rect = inputEl.getBoundingClientRect();
        labelAcDrop.style.top = (rect.bottom + 2) + 'px';
        labelAcDrop.style.left = rect.left + 'px';
        labelAcDrop.style.width = Math.max(rect.width, 160) + 'px';
        labelAcDrop.style.display = '';
      } else {
        if (labelAcDrop) { labelAcDrop.style.display = 'none'; }
      }
    });

    inputEl.addEventListener('blur', function () {
      setTimeout(function () {
        if (labelAcDrop) { labelAcDrop.style.display = 'none'; }
      }, 150);
    });

    inputEl.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && labelAcDrop) {
        labelAcDrop.style.display = 'none';
      }
    });
  }

  /* ── Multi-select label picker ───────────────────────────────────────── */
  function initLabelPicker(inputEl, presets) {
    if (inputEl._labelPickerBound) { return; }
    inputEl._labelPickerBound = true;

    var selected = inputEl.value
      ? inputEl.value.split(',').map(function(s){ return s.trim(); }).filter(Boolean)
      : [];

    inputEl.style.display = 'none';

    var wrap = document.createElement('div');
    wrap.className = 'lp-wrap';
    if (inputEl.classList.contains('view-pill-input')) { wrap.classList.add('lp-pill'); }

    var trigger = document.createElement('div');
    trigger.className = 'lp-trigger';

    var searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.className = 'lp-search';
    searchInput.autocomplete = 'off';
    searchInput.placeholder = selected.length ? '' : (inputEl.placeholder || 'Labels…');
    trigger.appendChild(searchInput);

    var drop = document.createElement('div');
    drop.className = 'lp-drop';
    drop.style.display = 'none';
    document.body.appendChild(drop);

    inputEl.parentNode.insertBefore(wrap, inputEl);
    wrap.appendChild(trigger);

    function syncInput() {
      inputEl.value = selected.join(', ');
      inputEl.dispatchEvent(new Event('input', {bubbles: true}));
      inputEl.dispatchEvent(new Event('change', {bubbles: true}));
    }

    function isSelected(label) {
      return selected.some(function(s) { return s.toLowerCase() === label.toLowerCase(); });
    }

    function renderTrigger() {
      Array.from(trigger.querySelectorAll('.lp-badge')).forEach(function(b) { trigger.removeChild(b); });
      selected.forEach(function(label) {
        var badge = document.createElement('span');
        badge.className = 'lp-badge';
        var txt = document.createTextNode(label + ' ');
        badge.appendChild(txt);
        var x = document.createElement('button');
        x.type = 'button';
        x.className = 'lp-badge-x';
        x.textContent = '×';
        x.addEventListener('mousedown', function(e) { e.preventDefault(); removeLabel(label); });
        badge.appendChild(x);
        trigger.insertBefore(badge, searchInput);
      });
      searchInput.placeholder = selected.length ? '' : (inputEl.placeholder || 'Labels…');
    }

    function addLabel(label) {
      label = label.trim();
      if (!label || isSelected(label)) { return; }
      var canonical = presets.find(function(p) { return p.toLowerCase() === label.toLowerCase(); });
      selected.push(canonical || label);
      renderTrigger();
      syncInput();
      searchInput.value = '';
      renderDrop();
    }

    function removeLabel(label) {
      selected = selected.filter(function(s) { return s.toLowerCase() !== label.toLowerCase(); });
      renderTrigger();
      syncInput();
      renderDrop();
    }

    function renderDrop() {
      var filter = searchInput.value.trim().toLowerCase();
      var filtered = filter
        ? presets.filter(function(p) { return p.toLowerCase().indexOf(filter) !== -1; })
        : presets;

      drop.innerHTML = '';
      filtered.forEach(function(label) {
        var sel = isSelected(label);
        var item = document.createElement('div');
        item.className = 'lp-item' + (sel ? ' lp-item-selected' : '');
        var chk = document.createElement('span');
        chk.className = 'lp-chk';
        chk.textContent = sel ? '☑' : '☐';
        item.appendChild(chk);
        var lbl = document.createElement('span');
        lbl.textContent = label;
        item.appendChild(lbl);
        item.addEventListener('mousedown', function(e) {
          e.preventDefault();
          if (isSelected(label)) { removeLabel(label); } else { addLabel(label); }
          searchInput.focus();
        });
        drop.appendChild(item);
      });

      var typedVal = searchInput.value.trim();
      if (typedVal && !isSelected(typedVal) && !presets.some(function(p) {
        return p.toLowerCase() === typedVal.toLowerCase();
      })) {
        var addItem = document.createElement('div');
        addItem.className = 'lp-item lp-add';
        addItem.textContent = '+ Add "' + typedVal + '"';
        addItem.addEventListener('mousedown', function(e) {
          e.preventDefault();
          addLabel(typedVal);
          searchInput.focus();
        });
        drop.appendChild(addItem);
      }

      var rect = trigger.getBoundingClientRect();
      drop.style.top = (rect.bottom + 2) + 'px';
      drop.style.left = rect.left + 'px';
      drop.style.minWidth = Math.max(rect.width, 180) + 'px';
    }

    function openDrop() { renderDrop(); drop.style.display = ''; }
    function closeDrop() { drop.style.display = 'none'; }

    searchInput.addEventListener('focus', openDrop);
    searchInput.addEventListener('input', renderDrop);
    searchInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') { e.preventDefault(); var v = searchInput.value.trim(); if (v) { addLabel(v); } }
      else if (e.key === 'Escape') { closeDrop(); }
      else if (e.key === 'Backspace' && !searchInput.value && selected.length) {
        removeLabel(selected[selected.length - 1]);
      }
    });
    searchInput.addEventListener('blur', function() { setTimeout(closeDrop, 150); });

    trigger.addEventListener('mousedown', function(e) {
      if (e.target !== searchInput) { e.preventDefault(); searchInput.focus(); }
    });

    function outsideClick(e) {
      var path = e.composedPath ? e.composedPath() : (e.path || []);
      var inside = path.length
        ? (path.indexOf(drop) !== -1 || path.indexOf(wrap) !== -1)
        : (wrap.contains(e.target) || drop.contains(e.target));
      if (!inside) { closeDrop(); }
    }
    document.addEventListener('mousedown', outsideClick);

    renderTrigger();
  }

  /* ── Quick-add show/hide ─────────────────────────────────────────────────── */
  var quickAddBar  = document.getElementById('quickAddBar');
  var quickAddHint = document.getElementById('quickAddHint');
  var qaDescEl     = document.getElementById('quickAddDesc');

  /* ── Contenteditable quick-add proxy ───────────────────────────────────── */
  var quickAddInput = (function () {
    var div    = document.getElementById('quickAddInput');
    var hidden = document.getElementById('quickAddInputValue');

    function renderContent(md) {
      var re = /\[(.+)\]\(((?:https?:\/\/)?[^\s)]+)\)/g;
      div.innerHTML = '';
      var last = 0, m;
      while ((m = re.exec(md)) !== null) {
        if (m.index > last) {
          div.appendChild(document.createTextNode(md.slice(last, m.index)));
        }
        var span = document.createElement('span');
        span.className = 'qa-md-link';
        span.contentEditable = 'false';
        span.dataset.md = m[0];
        var a = document.createElement('a');
        a.href = /^https?:\/\//.test(m[2]) ? m[2] : 'https://' + m[2];
        a.target = '_blank'; a.rel = 'noopener noreferrer';
        a.textContent = m[1];
        a.addEventListener('click', function (e) { e.stopPropagation(); });
        span.appendChild(a);
        div.appendChild(span);
        last = m.index + m[0].length;
      }
      if (last < md.length) {
        div.appendChild(document.createTextNode(md.slice(last)));
      }
      if (div.lastChild && div.lastChild.tagName === 'BR') {
        div.removeChild(div.lastChild);
      }
    }

    function extractMarkdown() {
      var result = '';
      div.childNodes.forEach(function (node) {
        if (node.nodeType === 3) {
          result += node.textContent;
        } else if (node.classList && node.classList.contains('qa-md-link')) {
          result += node.dataset.md || '';
        } else if (node.tagName !== 'BR') {
          result += node.textContent;
        }
      });
      return result;
    }

    function sync() {
      var md = extractMarkdown();
      hidden.value = md;
      return md;
    }

    function getCaretOffset() {
      var sel = window.getSelection();
      if (!sel || sel.rangeCount === 0) { return 0; }
      var range = sel.getRangeAt(0);
      var offset = 0;
      for (var i = 0; i < div.childNodes.length; i++) {
        var node = div.childNodes[i];
        if (range.startContainer === div && range.startOffset === i) { break; }
        if (range.startContainer === node) {
          if (node.nodeType === 3) { offset += range.startOffset; }
          break;
        }
        if (node.nodeType === 3) {
          offset += node.textContent.length;
        } else if (node.classList && node.classList.contains('qa-md-link')) {
          offset += (node.dataset.md || '').length;
        }
      }
      return offset;
    }

    function setCaretOffset(target) {
      var remaining = target;
      var range = document.createRange();
      var placed = false;
      for (var i = 0; i < div.childNodes.length; i++) {
        var node = div.childNodes[i];
        if (node.nodeType === 3) {
          if (remaining <= node.textContent.length) {
            range.setStart(node, remaining);
            range.collapse(true);
            placed = true;
            break;
          }
          remaining -= node.textContent.length;
        } else if (node.classList && node.classList.contains('qa-md-link')) {
          var mdLen = (node.dataset.md || '').length;
          if (remaining <= mdLen) {
            range.setStartAfter(node);
            range.collapse(true);
            placed = true;
            break;
          }
          remaining -= mdLen;
        }
      }
      if (!placed) { range.selectNodeContents(div); range.collapse(false); }
      var sel = window.getSelection();
      if (sel) { sel.removeAllRanges(); sel.addRange(range); }
    }

    div.addEventListener('input', function () {
      if (div.innerHTML === '<br>') { div.innerHTML = ''; }
    });

    return {
      get value() { return hidden.value; },
      set value(v) { hidden.value = v; renderContent(v); },
      get selectionStart() { return getCaretOffset(); },
      get selectionEnd()   { return getCaretOffset(); },
      setSelectionRange: function (s) { setCaretOffset(s); },
      focus: function () { div.focus(); },
      addEventListener: function (t, h) { div.addEventListener(t, h); },
      _div: div,
      _sync: sync,
      _render: renderContent
    };
  }());
  var qaPriorityEl  = document.getElementById('qaPriority');
  var qaLabelEl     = document.getElementById('qaLabel');
  var qaDueEl       = document.getElementById('qaDue');
  var qaDeadlineEl  = document.getElementById('qaDeadline');
  var qaSectionEl   = document.getElementById('qaSection');

  function showQuickAdd() {
    quickAddBar.style.display  = '';
    if (quickAddHint) { quickAddHint.style.display = 'none'; }
    syncControlsFromDetections(parseShortcutRanges(quickAddInput.value).detections);
    setTimeout(function () { quickAddInput.focus(); }, 0);
  }

  window.hideQuickAdd = function () {
    acClose();
    quickAddBar.style.display  = 'none';
    if (quickAddHint) { quickAddHint.style.display = ''; }
    quickAddInput.value = '';
    qaDescEl.value     = '';
    qaPriorityEl.value = '';
    qaLabelEl.value    = '';
    qaDueEl.value      = '';
    qaDeadlineEl.value = '';
    qaSectionEl.value  = '';
    qaUrlNote.style.display = 'none';
    qaUrlNote.textContent   = '';
  };

  /* ── Subtask toggle (client-side state flip after HTMX post) ────────────── */
  window.todoToggleSubtask = function (form) {
    var btn     = form.querySelector('button[data-done]');
    var wasDone = btn.dataset.done === '1';
    var isDone  = !wasDone;
    btn.dataset.done = isDone ? '1' : '0';
    btn.classList.toggle('subtask-check-done', isDone);
    var row = form.closest('.subtask-row');
    if (row) {
      row.classList.toggle('subtask-row-done', isDone);
      var titleEl = row.querySelector('.subtask-title');
      if (titleEl) {
        titleEl.classList.toggle('text-decoration-line-through', isDone);
        titleEl.classList.toggle('text-muted', isDone);
      }
    }
  };

  window.toggleDoneSubtasks = function (taskID) {
    var section = document.getElementById('subtask-section-' + taskID);
    var btn = document.getElementById('subtask-done-toggle-' + taskID);
    if (!section || !btn) { return; }
    var nowHiding = section.classList.toggle('hide-done');
    var count = section.querySelectorAll('.subtask-row-done').length;
    btn.textContent = nowHiding
      ? 'show done (' + count + ')'
      : 'hide done (' + count + ')';
  };

  /* ── Overtime counter ────────────────────────────────────────────────────── */
  var OT_KEY     = 'todos:overtime:' + (ACTIVE_WORKSPACE_ID || 'personal');
  var otDisplay  = document.getElementById('otDisplay');
  var otInput    = document.getElementById('otInput');

  function otGet() {
    return parseInt(localStorage.getItem(OT_KEY) || '0', 10);
  }
  function otSet(min) {
    localStorage.setItem(OT_KEY, String(min));
    otRender(min);
  }
  function otRender(min) {
    var neg = min < 0;
    var abs = Math.abs(min);
    var h   = Math.floor(abs / 60);
    var m   = abs % 60;
    otDisplay.textContent =
      (neg ? '−' : '+') + h + 'h ' + (m < 10 ? '0' : '') + m + 'm';
    otDisplay.style.color = neg
      ? 'var(--bs-danger)'
      : min > 0 ? 'var(--bs-success)' : '';
    otDisplay.style.borderColor = neg
      ? 'var(--bs-danger)' : min > 0 ? 'var(--bs-success)' : '';
  }
  function otAdjust(delta) { otSet(otGet() + delta); }

  document.getElementById('otDec60')
    .addEventListener('click', function () { otAdjust(-60); });
  document.getElementById('otDec15')
    .addEventListener('click', function () { otAdjust(-15); });
  document.getElementById('otInc15')
    .addEventListener('click', function () { otAdjust(15); });
  document.getElementById('otInc60')
    .addEventListener('click', function () { otAdjust(60); });
  document.getElementById('otReset')
    .addEventListener('click', function () { otSet(0); });

  otDisplay.addEventListener('click', function () {
    otInput.value           = otGet();
    otDisplay.style.display = 'none';
    otInput.style.display   = '';
    otInput.focus();
    otInput.select();
  });
  otInput.addEventListener('blur', function () {
    var v = parseInt(otInput.value, 10);
    if (!isNaN(v)) { otSet(v); }
    otInput.style.display   = 'none';
    otDisplay.style.display = '';
  });
  otInput.addEventListener('keydown', function (e) {
    if (e.key === 'Enter')  { otInput.blur(); }
    if (e.key === 'Escape') {
      otInput.style.display   = 'none';
      otDisplay.style.display = '';
    }
  });

  otRender(otGet());

  /* ── Autocomplete ────────────────────────────────────────────────────────── */
  var acDrop    = document.getElementById('acDropdown');
  var acItems   = [];
  var acIndex   = -1;
  var acTrigger = null;
  var acStart   = -1;

  function acOptions(filter) {
    filter = filter.toLowerCase();
    return filter ? PRESET_LABELS.filter(function (v) {
      return v.toLowerCase().startsWith(filter);
    }) : PRESET_LABELS;
  }

  function acSectionOptions(filter) {
    filter = filter.toLowerCase();
    var names = PRESET_SECTIONS.map(function (s) { return s.name; });
    return filter ? names.filter(function (v) {
      return v.toLowerCase().startsWith(filter);
    }) : names;
  }

  function acDateOptions(filter) {
    var opts = [
      'today', 'tomorrow',
      'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday',
      'next monday', 'next tuesday', 'next wednesday', 'next thursday',
      'next friday', 'next saturday', 'next sunday',
      'every monday', 'every tuesday', 'every wednesday', 'every thursday',
      'every friday', 'every saturday', 'every sunday',
      'every first sunday', 'every last friday'
    ];
    var f = filter.toLowerCase();
    return f ? opts.filter(function (v) {
      return v.toLowerCase().startsWith(f);
    }) : opts;
  }

  function acGetAnchorEl() {
    if (acTrigger === '@') { return qaLabelEl; }
    if (acTrigger === '#') { return qaSectionEl; }
    if (acTrigger === '!') { return qaDeadlineEl; }
    return quickAddInput;
  }

  function acPositionBelow(el) {
    var rect = el.getBoundingClientRect();
    acDrop.style.top   = (rect.bottom + 2) + 'px';
    acDrop.style.left  = rect.left + 'px';
    acDrop.style.width = Math.max(rect.width, 160) + 'px';
  }

  function acRender(opts) {
    acDrop.innerHTML = '';
    acItems = opts;
    acIndex = -1;
    opts.forEach(function (val, i) {
      var li = document.createElement('li');
      li.className = 'list-group-item';
      li.textContent = acTrigger === 'date' ? val : (acTrigger + val);
      li.addEventListener('mousedown', function (e) {
        e.preventDefault();
        acSelect(i);
      });
      acDrop.appendChild(li);
    });
    if (opts.length) {
      acPositionBelow(acGetAnchorEl());
      acDrop.style.display = '';
    } else {
      acDrop.style.display = 'none';
    }
  }

  function acHighlight() {
    Array.from(acDrop.children).forEach(function (li, i) {
      li.classList.toggle('active', i === acIndex);
    });
  }

  function acSelect(i) {
    var val = acItems[i];
    if (val == null) { return; }
    var cur   = quickAddInput.value;
    var after = cur.slice(quickAddInput.selectionStart);
    var replacement = acTrigger === 'date' ? val : (acTrigger + val);
    quickAddInput.value =
      cur.slice(0, acStart) + replacement + ' ' + after.trimStart();
    var newPos = acStart + replacement.length + 1;
    quickAddInput.setSelectionRange(newPos, newPos);
    acClose();
  }

  function acClose() {
    acDrop.style.display = 'none';
    acItems  = [];
    acIndex  = -1;
    acTrigger = null;
    acStart   = -1;
  }

  function acUpdate() {
    var val = quickAddInput.value;
    var pos = quickAddInput.selectionStart;
    var i = pos - 1;
    while (i >= 0 && val[i] !== ' ') { i--; }
    var wordStart = i + 1;
    var word = val.slice(wordStart, pos);
    if (word.startsWith('@')) {
      acTrigger = '@';
      acStart   = wordStart;
      acRender(acOptions(word.slice(1)));
    } else if (word.startsWith('#')) {
      acTrigger = '#';
      acStart   = wordStart;
      acRender(acSectionOptions(word.slice(1)));
    } else if (word.startsWith('!')) {
      acTrigger = '!';
      acStart   = wordStart;
      acRender(acDateOptions(word.slice(1)));
    } else {
      acClose();
    }
  }

  /* ── Quick-add inline highlighting ───────────────────────────────────────── */
  function parseShortcutRanges(val) {
    var ranges = [];
    var detections = [];
    var words = [];
    var match;
    var rx = /\S+/g;
    while ((match = rx.exec(val)) !== null) {
      words.push({
        raw: match[0],
        lower: match[0].toLowerCase(),
        start: match.index,
        end: match.index + match[0].length
      });
    }

    var mdLinkRe = /\[(.+)\]\([^)]*\)/g;
    var mdTitleRanges = [];
    while ((match = mdLinkRe.exec(val)) !== null) {
      mdTitleRanges.push({ start: match.index + 1, end: match.index + 1 + match[1].length });
    }
    function isInMdTitle(pos) {
      for (var k = 0; k < mdTitleRanges.length; k++) {
        if (pos >= mdTitleRanges[k].start && pos < mdTitleRanges[k].end) { return true; }
      }
      return false;
    }

    var weekdays = {
      monday: true, tuesday: true, wednesday: true, thursday: true,
      friday: true, saturday: true, sunday: true
    };
    var ordinals = {
      first: true, second: true, third: true, fourth: true, fifth: true, last: true
    };

    for (var i = 0; i < words.length; i++) {
      var w = words[i].lower;
      if (!w) { continue; }
      if (isInMdTitle(words[i].start)) { continue; }
      if (w[0] === '@' && w.length > 1) {
        var label = words[i].raw.slice(1);
        var canonicalLabel = PRESET_LABELS.find(function (l) {
          return l.toLowerCase() === label.toLowerCase();
        });
        var displayLabel = canonicalLabel || label;
        ranges.push({ start: words[i].start, end: words[i].end, kind: 'label', display: '@' + displayLabel });
        detections.push({kind: 'label', value: displayLabel});
      } else if (w[0] === '#' && w.length > 1) {
        var section = words[i].raw.slice(1);
        var canonicalSection = PRESET_SECTIONS.find(function (s) {
          return s.name.toLowerCase() === section.toLowerCase();
        });
        var displaySection = canonicalSection ? canonicalSection.name : section;
        ranges.push({ start: words[i].start, end: words[i].end, kind: 'section', display: '#' + displaySection });
        detections.push({kind: 'section', value: displaySection});
      } else if (/^p[123]$/i.test(w)) {
        ranges.push({start: words[i].start, end: words[i].end, kind: 'priority'});
        detections.push({kind: 'priority', value: w.toUpperCase()});
      } else if (w[0] === '!' && w.length > 1) {
        var dl = words[i].raw.slice(1).toLowerCase();
        var startIx = words[i].start;
        var endIx = words[i].end;
        if (dl === 'next' && i + 1 < words.length) {
          dl += ' ' + words[i + 1].raw.toLowerCase();
          endIx = words[i + 1].end;
          i++;
        }
        ranges.push({start: startIx, end: endIx, kind: 'deadline'});
        detections.push({kind: 'deadline', value: dl});
      } else if (w === 'every') {
        if (i + 2 < words.length && ordinals[words[i + 1].lower] && weekdays[words[i + 2].lower]) {
          ranges.push({start: words[i].start, end: words[i + 2].end, kind: 'recurring'});
          detections.push({ kind: 'recurring', value: 'every ' + words[i + 1].lower + ' ' + words[i + 2].lower });
          i += 2;
        } else if (i + 1 < words.length && weekdays[words[i + 1].lower]) {
          ranges.push({start: words[i].start, end: words[i + 1].end, kind: 'recurring'});
          detections.push({kind: 'recurring', value: 'every ' + words[i + 1].lower});
          i += 1;
        } else if (i + 2 < words.length && /^\d+$/.test(words[i + 1].raw) && /^days?$/i.test(words[i + 2].raw)) {
          ranges.push({start: words[i].start, end: words[i + 2].end, kind: 'recurring'});
          detections.push({ kind: 'recurring', value: 'every ' + words[i + 1].raw + ' ' + words[i + 2].lower });
          i += 2;
        }
      } else {
        var w2 = i + 1 < words.length ? words[i + 1].lower : '';
        if (w === 'next' && weekdays[w2]) {
          ranges.push({start: words[i].start, end: words[i + 1].end, kind: 'due'});
          detections.push({kind: 'due', value: 'next ' + w2});
          i++;
        } else if (w === 'today' || w === 'tomorrow' || weekdays[w]) {
          ranges.push({start: words[i].start, end: words[i].end, kind: 'due'});
          detections.push({kind: 'due', value: w});
        } else if (/^\d{4}-\d{2}-\d{2}$/.test(w)) {
          ranges.push({start: words[i].start, end: words[i].end, kind: 'due'});
          detections.push({kind: 'due', value: w});
        }
      }
    }
    return { ranges: ranges, detections: detections };
  }

  function syncControlsFromDetections(items) {
    qaPriorityEl.value = '';
    qaLabelEl.value    = '';
    qaDueEl.value      = '';
    qaDeadlineEl.value = '';
    qaSectionEl.value  = '';
    var labelValues = [];
    items.forEach(function (d) {
      if (d.kind === 'priority')  { qaPriorityEl.value = d.value.toLowerCase(); }
      if (d.kind === 'label')     { labelValues.push(d.value); }
      if (d.kind === 'due')       { qaDueEl.value      = d.value; }
      if (d.kind === 'recurring') { qaDueEl.value      = d.value; }
      if (d.kind === 'deadline')  { qaDeadlineEl.value = d.value; }
      if (d.kind === 'section')   { qaSectionEl.value  = d.value; }
    });
    qaLabelEl.value = labelValues.join(', ');
  }

  function removeKindFromText(val, kind) {
    var parsed = parseShortcutRanges(val);
    var toRemove = parsed.ranges.filter(function (r) { return r.kind === kind; });
    if (!toRemove.length) { return val; }
    toRemove.sort(function (a, b) { return b.start - a.start; });
    toRemove.forEach(function (r) {
      val = (val.slice(0, r.start) + ' ' + val.slice(r.end))
        .replace(/\s+/g, ' ').trim();
    });
    return val;
  }

  function applyControlToText(kind, shortcut) {
    var val = removeKindFromText(quickAddInput.value, kind);
    if (shortcut) { val = (val + ' ' + shortcut).trim(); }
    quickAddInput.value = val;
    syncControlsFromDetections(parseShortcutRanges(val).detections);
  }

  qaPriorityEl.addEventListener('change', function () {
    applyControlToText('priority', qaPriorityEl.value);
    quickAddInput.focus();
  });
  (function () {
    function commitLabel() {
      var v = qaLabelEl.value.trim();
      var parts = v ? v.split(',') : [];
      var shortcuts = parts.map(function (l) {
        return l.trim() ? ('@' + l.trim()) : '';
      }).filter(Boolean).join(' ');
      applyControlToText('label', shortcuts);
    }
    qaLabelEl.addEventListener('change', commitLabel);
    qaLabelEl.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); commitLabel(); quickAddInput.focus(); }
      if (e.key === 'Escape') { qaLabelEl.value = ''; commitLabel(); quickAddInput.focus(); }
    });
  }());
  initLabelPicker(qaLabelEl, PRESET_LABELS);
  (function () {
    function commitSection() {
      var v = qaSectionEl.value.trim();
      applyControlToText('section', v ? ('#' + v) : '');
    }
    qaSectionEl.addEventListener('change', commitSection);
    qaSectionEl.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); commitSection(); quickAddInput.focus(); }
      if (e.key === 'Escape') { qaSectionEl.value = ''; commitSection(); quickAddInput.focus(); }
    });
  }());
  (function () {
    function commitDue() { applyControlToText('due', qaDueEl.value.trim()); }
    qaDueEl.addEventListener('change', commitDue);
    qaDueEl.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); commitDue(); quickAddInput.focus(); }
      if (e.key === 'Escape') { qaDueEl.value = ''; commitDue(); quickAddInput.focus(); }
    });
  }());
  (function () {
    function commitDeadline() {
      var v = qaDeadlineEl.value.trim();
      if (v && !v.startsWith('!')) { v = '!' + v; }
      applyControlToText('deadline', v);
    }
    qaDeadlineEl.addEventListener('change', commitDeadline);
    qaDeadlineEl.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); commitDeadline(); quickAddInput.focus(); }
      if (e.key === 'Escape') { qaDeadlineEl.value = ''; commitDeadline(); quickAddInput.focus(); }
    });
  }());

  var qaUrlNote = document.getElementById('qaUrlNote');

  function matchPattern(rawURL) {
    for (var j = 0; j < URL_PATTERNS.length; j++) {
      if (rawURL.startsWith(URL_PATTERNS[j].prefix)) { return URL_PATTERNS[j]; }
    }
    return null;
  }

  function extractIDFromURL(rawURL, pattern) {
    if (!pattern) { return ''; }
    var rest = rawURL.slice(pattern.prefix.length);
    return rest.split(/[/?#]/)[0] || '';
  }

  function extractURLFromInput(val) {
    var mdMatch = /\]\((https?:\/\/[^)]+)\)/.exec(val);
    if (mdMatch) { return mdMatch[1]; }
    var words = val.trim().split(/\s+/);
    for (var i = 0; i < words.length; i++) {
      if (words[i].startsWith('http://') || words[i].startsWith('https://')) {
        return words[i];
      }
    }
    return null;
  }

  function updateUrlIndicator(val) {
    var url = extractURLFromInput(val);
    if (!url) { qaUrlNote.style.display = 'none'; return; }
    var pattern = matchPattern(url);
    if (!pattern) { qaUrlNote.style.display = 'none'; return; }
    var id  = extractIDFromURL(url, pattern);
    var ref = pattern.shortcut ? (pattern.shortcut + id) : (id ? '#' + id : '');
    if (!ref) { qaUrlNote.style.display = 'none'; return; }
    qaUrlNote.textContent = pattern.platform + ' · ' + ref;
    qaUrlNote.style.display = '';
  }

  quickAddInput.addEventListener('paste', function (e) {
    var html = e.clipboardData && e.clipboardData.getData('text/html');
    if (html) {
      var tmp = document.createElement('div');
      tmp.innerHTML = html;
      var anchor = tmp.querySelector('a[href]');
      if (anchor) {
        var url   = anchor.href;
        var title = anchor.textContent.trim();
        if (url && title && title !== url) {
          e.preventDefault();
          var pattern = matchPattern(url);
          var id      = extractIDFromURL(url, pattern);
          var ref     = pattern && pattern.shortcut
            ? pattern.shortcut + id : (id ? '#' + id : '');
          if (ref && title.indexOf(id) === -1) { title += ' ' + ref; }
          var replacement = '[' + title + '](' + url + ')';
          var start = quickAddInput.selectionStart;
          var end   = quickAddInput.selectionEnd;
          quickAddInput.value =
            (quickAddInput.value.slice(0, start) + replacement +
             quickAddInput.value.slice(end)).trim();
          var richNewPos = start + replacement.length;
          setTimeout(function () {
            quickAddInput.focus();
            quickAddInput.setSelectionRange(
              Math.min(richNewPos, quickAddInput.value.length)
            );
          }, 0);
          syncControlsFromDetections(
            parseShortcutRanges(quickAddInput.value).detections
          );
          updateUrlIndicator(quickAddInput.value);
          return;
        }
      }
    }
    setTimeout(function () {
      var md = quickAddInput._sync();
      if (/\[.+\]\((?:https?:\/\/)?[^\s)]+\)/.test(md)) {
        var caretPos = quickAddInput.selectionStart;
        quickAddInput._render(md);
        setTimeout(function () {
          quickAddInput.focus();
          quickAddInput.setSelectionRange(caretPos);
        }, 0);
      }
      updateUrlIndicator(quickAddInput.value);
    }, 0);
  });

  quickAddInput.addEventListener('input', function () {
    var md = quickAddInput._sync();
    if (/\[.+\]\((?:https?:\/\/)?[^\s)]+\)/.test(md)) {
      var caretPos = quickAddInput.selectionStart;
      quickAddInput._render(md);
      setTimeout(function () {
        quickAddInput.focus();
        quickAddInput.setSelectionRange(caretPos);
      }, 0);
    }
    acUpdate();
    syncControlsFromDetections(parseShortcutRanges(md).detections);
    updateUrlIndicator(md);
  });
  quickAddInput.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') { e.preventDefault(); }

    if (acDrop.style.display === 'none') {
      if (e.key === 'Escape') { hideQuickAdd(); }
      if (e.key === 'Enter') {
        quickAddInput._sync();
        var btn = quickAddInput._div.closest('form')
          .querySelector('button[type="submit"]');
        if (btn) { btn.click(); }
      }
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      acIndex = Math.min(acIndex + 1, acItems.length - 1);
      acHighlight();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      acIndex = Math.max(acIndex - 1, 0);
      acHighlight();
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      if (acIndex >= 0) {
        e.preventDefault();
        acSelect(acIndex);
      } else if (acItems.length === 1) {
        e.preventDefault();
        acSelect(0);
      } else {
        acClose();
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      acClose();
    }
  });
  quickAddInput.addEventListener('blur', function () {
    setTimeout(acClose, 150);
  });

  quickAddBar.addEventListener('focusout', function (e) {
    if (!quickAddBar.contains(e.relatedTarget)) {
      hideQuickAdd();
    }
  });

  /* ── Task keyboard navigation ────────────────────────────────────────────── */
  var kbIndex           = -1;
  var hoveredRow        = null;
  var activeParentTaskRow = null;

  function getTaskRows() {
    return Array.from(document.querySelectorAll('.task-row'));
  }

  function isSubtaskVisible(subtaskRow) {
    var el = subtaskRow.parentElement;
    while (el) {
      if (el.classList.contains('subtask-children') && el.style.display === 'none') {
        return false;
      }
      el = el.parentElement;
    }
    return true;
  }

  function getSubtasksForFocusedTask(taskRow) {
    if (!taskRow) return [];
    var subtaskList = taskRow.querySelector('.subtask-list[id^="subtasks-"]') ||
                      taskRow.querySelector('.subtask-children');
    if (!subtaskList) return [];
    return Array.from(subtaskList.querySelectorAll(':scope > .subtask-row'))
      .filter(function(row) { return isSubtaskVisible(row); });
  }

  function setKbFocus(idx) {
    getTaskRows().forEach(function (r) { r.classList.remove('kb-focus'); });
    kbIndex = idx;
    var rows = getTaskRows();
    if (idx >= 0 && idx < rows.length) {
      rows[idx].classList.add('kb-focus');
      rows[idx].scrollIntoView({ block: 'nearest' });
    }
  }

  function bindTaskRowEvents() {
    getTaskRows().forEach(function (row) {
      if (row._todosBound) { return; }
      row._todosBound = true;
      row.addEventListener('mouseenter', function () {
        hoveredRow = row;
        setKbFocus(-1);
      });
      row.addEventListener('mouseleave', function () { hoveredRow = null; });
    });
  }
  bindTaskRowEvents();

  /* ── Subtask quick-add ───────────────────────────────────────────────────── */
  function initSubtaskQA(form) {
    var div = form.querySelector('.subtask-qa-input');
    if (!div || div._subQABound) { return; }
    div._subQABound = true;
    var hidden = form.querySelector('.subtask-qa-hidden');
    var taskId = div.dataset.taskId;

    function sync() {
      var result = '';
      div.childNodes.forEach(function (node) {
        if (node.nodeType === 3) { result += node.textContent; }
        else if (node.tagName !== 'BR') { result += node.textContent; }
      });
      if (hidden) { hidden.value = result; }
      return result;
    }

    div.addEventListener('input', function () {
      if (div.innerHTML === '<br>') { div.innerHTML = ''; }
      sync();
    });
    div.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        if (sync().trim()) {
          var btn = form.querySelector('button[type="submit"]');
          if (btn) { btn.click(); }
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        window.subQaHide(taskId);
      }
    });

    form.addEventListener('focusout', function (e) {
      if (!form.contains(e.relatedTarget)) {
        window.subQaHide(taskId);
      }
    });
  }

  function initAllSubtaskQAs() {
    document.querySelectorAll('.subtask-qa-form').forEach(initSubtaskQA);
  }

  window.subQaToggle = function (taskId) {
    var form = document.getElementById('subtask-add-' + taskId);
    if (!form) { return; }
    if (form.style.display === 'none' || !form.style.display) {
      var section = document.getElementById('subtask-section-' + taskId);
      if (section) { section.classList.add('mt-2'); }
      form.style.display = '';
      initSubtaskQA(form);
      var ce = form.querySelector('.subtask-qa-input');
      if (ce) { setTimeout(function () { ce.focus(); }, 0); }
    } else {
      window.subQaHide(taskId);
    }
  };

  window.subQaHide = function (taskId) {
    var form = document.getElementById('subtask-add-' + taskId);
    if (!form) { return; }
    form.style.display = 'none';
    var ce = form.querySelector('.subtask-qa-input');
    if (ce) { ce.innerHTML = ''; }
    var desc = form.querySelector('[name="description"]');
    if (desc) { desc.value = ''; }
  };

  window.subQaAfterAdd = function (taskId) {
    var form = document.getElementById('subtask-add-' + taskId);
    if (!form) { return; }
    var ce = form.querySelector('.subtask-qa-input');
    if (ce) { ce.innerHTML = ''; ce.focus(); }
    var desc = form.querySelector('[name="description"]');
    if (desc) { desc.value = ''; }
    var taskRow = document.querySelector(
      '.task-row[data-task-id="' + taskId + '"]'
    );
    if (taskRow) {
      var list  = document.getElementById('subtasks-' + taskId);
      var total = list ? list.querySelectorAll('.subtask-row').length : 0;
      var done  = list
        ? list.querySelectorAll('.subtask-check-done').length : 0;
      var badge = taskRow.querySelector('.subtask-counter');
      if (!badge && total > 0) {
        var badgesDiv = taskRow.querySelector('.d-flex.flex-wrap.gap-1');
        if (badgesDiv) {
          badge = document.createElement('span');
          badge.className = 'badge bg-light text-dark border subtask-counter';
          badgesDiv.appendChild(badge);
        }
      }
      if (badge) { badge.textContent = done + '/' + total; }
    }
  };

  window.qeOpen = function (taskId) {
    var row = document.querySelector('.task-row[data-task-id="' + taskId + '"]');
    if (!row) { return; }
    var form = document.getElementById('quick-edit-' + taskId);
    if (!form) { return; }
    var descStore = row.querySelector('.task-desc-store');
    var descInput = form.querySelector('.qe-desc-src');
    if (descStore && descInput) { descInput.value = descStore.value; }
    form.style.display = '';
    var titleInput = form.querySelector('input[name="title"]');
    if (titleInput) { setTimeout(function () { titleInput.focus(); titleInput.select(); }, 0); }
    var oldLabelInput = form.querySelector('input[name="label"]');
    if (oldLabelInput) { initLabelPicker(oldLabelInput, PRESET_LABELS); }
  };

  window.qeClose = function (taskId) {
    var form = document.getElementById('quick-edit-' + taskId);
    if (form) { form.style.display = 'none'; }
  };

  /* ── Global keyboard handler ─────────────────────────────────────────────── */
  document.addEventListener('keydown', function (e) {
    var tag    = e.target.tagName;
    var inInput = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
        || e.target.isContentEditable;
    var modalOpen = !!document.querySelector('.modal.show');
    if (modalOpen) { return; }

    if (!inInput && e.key === '/') {
      e.preventDefault();
      showQuickAdd();
      return;
    }

    if (!inInput && (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
      var tabLinks = Array.from(
        document.querySelectorAll('.nav-tabs .nav-link[hx-get]')
      );
      var activeTabIdx = tabLinks.findIndex(function (l) {
        return l.classList.contains('active');
      });
      if (activeTabIdx !== -1) {
        var nextTabIdx = e.key === 'ArrowRight'
          ? activeTabIdx + 1 : activeTabIdx - 1;
        if (nextTabIdx >= 0 && nextTabIdx < tabLinks.length) {
          e.preventDefault();
          var nextTab = tabLinks[nextTabIdx];
          var nextHref = nextTab.getAttribute('hx-get');
          tabLinks.forEach(function (l) { l.classList.remove('active'); });
          nextTab.classList.add('active');
          syncQuickAddSection();
          htmx.ajax('GET', nextHref, {
            target: '#taskListContainer',
            swap: 'innerHTML'
          });
          history.pushState({}, '', nextHref);
        }
      }
      return;
    }

    if (!inInput && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      var isDown = e.key === 'ArrowDown';
      e.preventDefault();

      if (e.shiftKey) {
        var rows = getTaskRows();
        if (activeSubtaskRow !== null) {
          var parentTask = activeParentTaskRow || (kbIndex >= 0 ? rows[kbIndex] : null);
          var subtasks = parentTask ? getSubtasksForFocusedTask(parentTask) : [];
          var curIdx = subtasks.indexOf(activeSubtaskRow);
          var nextIdx = isDown ? curIdx + 1 : curIdx - 1;
          if (nextIdx >= 0 && nextIdx < subtasks.length) {
            setActiveSubtaskRow(subtasks[nextIdx]);
            activeParentTaskRow = parentTask;
          } else if (isDown) {
            setActiveSubtaskRow(null);
            activeParentTaskRow = null;
            var nextTask = kbIndex + 1;
            if (nextTask < rows.length) setKbFocus(nextTask);
          } else {
            setActiveSubtaskRow(null);
            activeParentTaskRow = null;
            if (kbIndex >= 0) setKbFocus(kbIndex);
          }
        } else {
          if (kbIndex < 0 && rows.length > 0) setKbFocus(0);
          if (kbIndex >= 0) {
            var subtasks = getSubtasksForFocusedTask(rows[kbIndex]);
            if (subtasks.length > 0) {
              activeParentTaskRow = rows[kbIndex];
              setActiveSubtaskRow(isDown ? subtasks[0] : subtasks[subtasks.length - 1]);
            }
          }
        }
      } else {
        if (activeSubtaskRow !== null) {
          setActiveSubtaskRow(null);
          activeParentTaskRow = null;
        }
        var rows = getTaskRows();
        if (rows.length === 0) return;
        var next;
        if (kbIndex === -1) {
          next = isDown ? 0 : rows.length - 1;
        } else {
          next = isDown ? Math.min(kbIndex + 1, rows.length - 1)
                        : Math.max(kbIndex - 1, 0);
        }
        setKbFocus(next);
      }
    }

    if (!inInput && e.key === 's') {
      if (activeSubtaskRow) {
        e.preventDefault();
        var depth = parseInt(activeSubtaskRow.dataset.subtaskDepth, 10);
        if (depth < 2) {
          var subtaskId = activeSubtaskRow.dataset.subtaskId;
          if (subtaskId) { window.showAddChild(subtaskId); }
        }
      } else {
        var rows2 = getTaskRows();
        var target = kbIndex >= 0 ? rows2[kbIndex] : hoveredRow;
        if (target) {
          e.preventDefault();
          var tId = target.dataset.taskId;
          if (tId) { window.subQaToggle(tId); }
        }
      }
    }

    if (!inInput && e.key === 'e') {
      if (activeSubtaskRow) {
        e.preventDefault();
        window.showSubtaskEdit(activeSubtaskRow.dataset.subtaskId);
      } else {
        var rows3 = getTaskRows();
        var target3 = kbIndex >= 0 ? rows3[kbIndex] : hoveredRow;
        if (target3) {
          e.preventDefault();
          var tId3 = target3.dataset.taskId;
          if (tId3) { window.taskQeOpen(tId3); }
        }
      }
    }

    if (!inInput && e.key === 'c' && activeSubtaskRow) {
      e.preventDefault();
      var sid = activeSubtaskRow.dataset.subtaskId;
      if (sid) { window.showAddChild(sid); }
    }
    if (!inInput && e.key === 'Escape') {
      setActiveSubtaskRow(null);
      activeParentTaskRow = null;
    }
  });

  /* ── Drag-drop reorder via SortableJS ───────────────────────────────────── */
  var sortableOpts = {
    handle: '.drag-handle',
    animation: 150,
    ghostClass: 'sortable-ghost',
    onEnd: function () {
      var el = document.getElementById('taskSortable');
      if (!el) { return; }
      var ids = Array.from(el.querySelectorAll('[data-task-id]'))
        .map(function (e) { return e.dataset.taskId; });
      fetch('/todos/reorder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: ids })
      });
    }
  };

  function initSortable() {
    var el = document.getElementById('taskSortable');
    if (el && !Sortable.get(el)) {
      Sortable.create(el, sortableOpts);
    }
  }

  function initSubtaskSortables() {
    document.querySelectorAll('.subtask-list[id^="subtasks-"]').forEach(
      function (el) {
        if (Sortable.get(el)) { return; }
        var taskId = el.id.slice('subtasks-'.length);
        Sortable.create(el, {
          handle: '.drag-handle-sub',
          animation: 150,
          ghostClass: 'sortable-ghost',
          onEnd: function () {
            var ids = Array.from(el.querySelectorAll('[data-subtask-id]'))
              .map(function (e) { return e.dataset.subtaskId; });
            fetch('/todos/' + taskId + '/subtasks/reorder', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ ids: ids })
            });
          }
        });
      }
    );
  }

  initSortable();
  initSubtaskSortables();
  initAllSubtaskQAs();

  /* ── Active-section helpers ──────────────────────────────────────────────── */
  function getActiveSectionId() {
    var activeLink = document.querySelector('.nav-tabs .nav-link.active[hx-get]');
    if (!activeLink) { return ''; }
    var href = activeLink.getAttribute('hx-get') || '';
    var m = /[?&]section=([^&]*)/.exec(href);
    return m ? m[1] : '';
  }

  function syncQuickAddSection() {
    var hiddenInput = document.querySelector('#quickAddBar input[name="section_id"]');
    if (hiddenInput) { hiddenInput.value = getActiveSectionId(); }
  }

  /* ── Re-init after HTMX swaps ────────────────────────────────────────────── */
  document.body.addEventListener('htmx:afterSettle', function () {
    initSortable();
    initSubtaskSortables();
    initAllSubtaskQAs();
    bindTaskRowEvents();
    syncQuickAddSection();
    kbIndex = -1;
    activeParentTaskRow = null;
    setActiveSubtaskRow(null);
  });

  /* ── Task inline quick-edit ──────────────────────────────────────────────── */
  function initTaskQe(qeDiv) {
    if (qeDiv._qeBound) { return; }
    qeDiv._qeBound = true;
    var taskId  = qeDiv.dataset.taskId;
    var ceDiv   = qeDiv.querySelector('.task-qe-input');
    var hidden  = qeDiv.querySelector('.task-qe-hidden');

    var priorityEl = qeDiv.querySelector('.task-qe-priority');
    var labelEl    = qeDiv.querySelector('.task-qe-label');
    var dueEl      = qeDiv.querySelector('.task-qe-due');
    var deadlineEl = qeDiv.querySelector('.task-qe-deadline');
    var sectionEl  = qeDiv.querySelector('.task-qe-section');

    function ceSync() {
      var result = '';
      ceDiv.childNodes.forEach(function (node) {
        if (node.nodeType === 3) { result += node.textContent; }
        else if (node.classList && node.classList.contains('qa-md-link')) {
          result += node.dataset.md || '';
        } else if (node.tagName !== 'BR') { result += node.textContent; }
      });
      hidden.value = result;
      return result;
    }

    function qeSyncControls(items) {
      priorityEl.value = '';
      labelEl.value    = '';
      dueEl.value      = '';
      deadlineEl.value = '';
      sectionEl.value  = '';
      var labelValues = [];
      items.forEach(function (d) {
        if (d.kind === 'priority')  { priorityEl.value = d.value.toLowerCase(); }
        if (d.kind === 'label')     { labelValues.push(d.value); }
        if (d.kind === 'due' || d.kind === 'recurring') { dueEl.value = d.value; }
        if (d.kind === 'deadline')  { deadlineEl.value = d.value; }
        if (d.kind === 'section')   { sectionEl.value  = d.value; }
      });
      labelEl.value = labelValues.join(', ');
    }

    function qeRemoveKind(val, kind) { return removeKindFromText(val, kind); }

    function qeApply(kind, shortcut) {
      var val = qeRemoveKind(hidden.value, kind);
      if (shortcut) { val = (val + ' ' + shortcut).trim(); }
      hidden.value = val;
      ceDiv.textContent = val;
      qeSyncControls(parseShortcutRanges(val).detections);
    }

    ceDiv.addEventListener('input', function () {
      if (ceDiv.innerHTML === '<br>') { ceDiv.innerHTML = ''; }
      qeSyncControls(parseShortcutRanges(ceSync()).detections);
    });

    ceDiv.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        if (ceSync().trim()) {
          var btn = qeDiv.querySelector('.task-qe-submit');
          if (btn) { btn.click(); }
        }
      }
      if (e.key === 'Escape') { e.preventDefault(); window.taskQeHide(taskId); }
    });

    priorityEl.addEventListener('change', function () {
      qeApply('priority', priorityEl.value);
      ceDiv.focus();
    });

    (function () {
      function commitLabel() {
        var v = labelEl.value.trim();
        var parts = v ? v.split(',') : [];
        var shortcuts = parts.map(function (l) {
          return l.trim() ? ('@' + l.trim()) : '';
        }).filter(Boolean).join(' ');
        qeApply('label', shortcuts);
      }
      labelEl.addEventListener('change', commitLabel);
      labelEl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter')  { e.preventDefault(); commitLabel(); ceDiv.focus(); }
        if (e.key === 'Escape') { labelEl.value = ''; commitLabel(); ceDiv.focus(); }
      });
    }());
    initLabelPicker(labelEl, PRESET_LABELS);

    (function () {
      function commitDue() { qeApply('due', dueEl.value.trim()); }
      dueEl.addEventListener('change', commitDue);
      dueEl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter')  { e.preventDefault(); commitDue(); ceDiv.focus(); }
        if (e.key === 'Escape') { dueEl.value = ''; commitDue(); ceDiv.focus(); }
      });
    }());

    (function () {
      function commitDeadline() {
        var v = deadlineEl.value.trim();
        if (v && !v.startsWith('!')) { v = '!' + v; }
        qeApply('deadline', v);
      }
      deadlineEl.addEventListener('change', commitDeadline);
      deadlineEl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); commitDeadline(); ceDiv.focus(); }
        if (e.key === 'Escape') { deadlineEl.value = ''; commitDeadline(); ceDiv.focus(); }
      });
    }());

    (function () {
      function commitSection() {
        var v = sectionEl.value.trim();
        qeApply('section', v ? ('#' + v) : '');
      }
      sectionEl.addEventListener('change', commitSection);
      sectionEl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); commitSection(); ceDiv.focus(); }
        if (e.key === 'Escape') { sectionEl.value = ''; commitSection(); ceDiv.focus(); }
      });
    }());

    qeDiv.addEventListener('focusout', function (e) {
      setTimeout(function () {
        if (qeDiv.style.display !== 'none' &&
            !qeDiv.contains(document.activeElement)) {
          window.taskQeHide(taskId);
        }
      }, 150);
    });
  }

  window.taskQeOpen = function (taskId) {
    document.querySelectorAll('.task-qe[style=""],.task-qe:not([style*="none"])')
      .forEach(function (el) {
        var otherId = el.dataset.taskId;
        if (otherId && otherId !== taskId) { window.taskQeHide(otherId); }
      });

    var row = document.querySelector('.task-row[data-task-id="' + taskId + '"]');
    if (!row) { return; }
    var qeDiv = document.getElementById('task-qe-' + taskId);
    if (!qeDiv) { return; }

    initTaskQe(qeDiv);

    var title    = row.dataset.title    || '';
    var priority = row.dataset.priority || '0';
    var labels   = row.dataset.labels   || '';
    var sectionId = row.dataset.sectionId || '';
    var due      = row.dataset.due      || '';
    var deadline = row.dataset.deadline || '';
    var recur    = row.dataset.recur    || '';

    var parts = [title];
    if (priority && priority !== '0') { parts.push('p' + priority); }
    labels.split(',').forEach(function (l) {
      if (l.trim()) { parts.push('@' + l.trim()); }
    });
    if (recur) {
      parts.push(recur);
    } else if (due) {
      parts.push(due);
    }
    if (deadline) { parts.push('!' + deadline); }
    var sect = PRESET_SECTIONS.find(function (s) { return s.id === sectionId; });
    if (sect) { parts.push('#' + sect.name); }

    var inputText = parts.filter(Boolean).join(' ');

    var ceDiv  = qeDiv.querySelector('.task-qe-input');
    var hidden = qeDiv.querySelector('.task-qe-hidden');
    hidden.value = inputText;
    ceDiv.textContent = inputText;

    var descStore = row.querySelector('.task-desc-store');
    var descEl    = qeDiv.querySelector('.task-qe-desc');
    if (descEl && descStore) { descEl.value = descStore.value; }

    var detections = parseShortcutRanges(inputText).detections;
    qeDiv.querySelector('.task-qe-priority').value = '';
    qeDiv.querySelector('.task-qe-label').value    = '';
    qeDiv.querySelector('.task-qe-due').value      = '';
    qeDiv.querySelector('.task-qe-deadline').value = '';
    qeDiv.querySelector('.task-qe-section').value  = '';
    var qeLabelValues = [];
    detections.forEach(function (d) {
      if (d.kind === 'priority') { qeDiv.querySelector('.task-qe-priority').value = d.value.toLowerCase(); }
      if (d.kind === 'label') { qeLabelValues.push(d.value); }
      if (d.kind === 'due' || d.kind === 'recurring') { qeDiv.querySelector('.task-qe-due').value = d.value; }
      if (d.kind === 'deadline') { qeDiv.querySelector('.task-qe-deadline').value = d.value; }
      if (d.kind === 'section') { qeDiv.querySelector('.task-qe-section').value = d.value; }
    });
    qeDiv.querySelector('.task-qe-label').value = qeLabelValues.join(', ');

    var mainContent = row.querySelector('.task-main-content');
    if (mainContent) { mainContent.style.display = 'none'; }
    qeDiv.style.display = '';
    setTimeout(function () { ceDiv.focus(); }, 0);
  };

  window.taskQeHide = function (taskId) {
    var row = document.querySelector('.task-row[data-task-id="' + taskId + '"]');
    if (!row) { return; }
    var qeDiv = document.getElementById('task-qe-' + taskId);
    if (qeDiv) { qeDiv.style.display = 'none'; }
    var mainContent = row.querySelector('.task-main-content');
    if (mainContent) { mainContent.style.display = ''; }
  };

  /* ── Sync nav-tab active class after HTMX pushes a new URL ──────────────── */
  function syncActiveTab() {
    var currentHref = window.location.pathname + window.location.search;
    document.querySelectorAll('.nav-tabs .nav-link').forEach(function (l) {
      l.classList.toggle('active', l.getAttribute('href') === currentHref);
    });
  }
  document.body.addEventListener('htmx:pushedIntoHistory', syncActiveTab);

  /* ── Optimistic active-tab highlight on click + loading dim ─────────────── */
  var taskListContainer = document.getElementById('taskListContainer');
  document.querySelectorAll('.nav-tabs .nav-link[hx-get]').forEach(function (tab) {
    tab.addEventListener('click', function () {
      document.querySelectorAll('.nav-tabs .nav-link').forEach(function (l) {
        l.classList.remove('active');
      });
      tab.classList.add('active');
      if (taskListContainer) { taskListContainer.style.opacity = '0.4'; }
    });
  });
  document.body.addEventListener('htmx:afterSettle', function (e) {
    if (e.detail && e.detail.target && e.detail.target.id === 'taskListContainer') {
      if (taskListContainer) { taskListContainer.style.opacity = ''; }
    }
  });

  /* ── Inline description editor ───────────────────────────────────────────── */
  var descView        = document.getElementById('modalDescView');
  var descEl          = document.getElementById('modalDesc');
  var descLivePreview = document.getElementById('modalDescLivePreview');

  var PLACEHOLDER = '<span class="text-muted fst-italic small">'
    + 'Click to add a description…</span>';

  function renderDesc() {
    var src = descEl.value.trim();
    descView.innerHTML = src ? marked.parse(src) : PLACEHOLDER;
  }

  function updateLivePreview() {
    var src = descEl.value.trim();
    descLivePreview.innerHTML = src ? marked.parse(src) : '';
    descLivePreview.style.display = src ? '' : 'none';
  }

  function startEdit() {
    descView.style.display = 'none';
    descEl.style.display   = '';
    descEl.style.height    = 'auto';
    descEl.style.height    = Math.max(120, descEl.scrollHeight) + 'px';
    updateLivePreview();
    descEl.focus();
  }

  function endEdit() {
    renderDesc();
    descEl.style.display          = 'none';
    descLivePreview.style.display = 'none';
    descView.style.display        = '';
  }

  descView.addEventListener('click', startEdit);
  descEl.addEventListener('blur', endEdit);
  descEl.addEventListener('input', function () {
    this.style.height = 'auto';
    this.style.height = Math.max(120, this.scrollHeight) + 'px';
    updateLivePreview();
  });

  /* ── Task modal ──────────────────────────────────────────────────────────── */
  var modal        = document.getElementById('taskModal');
  var form         = document.getElementById('taskForm');
  var titleEl      = document.getElementById('modalTitle');
  var priorityEl   = document.getElementById('modalPriority');
  var labelEl      = document.getElementById('modalLabel');
  var dueEl        = document.getElementById('modalDue');
  var deadlineEl   = document.getElementById('modalDeadline');
  var sectionEl    = document.getElementById('modalSection');
  var recurDaysEl  = document.getElementById('modalRecurDays');
  var lbl          = document.getElementById('taskModalLabel');

  modal.addEventListener('show.bs.modal', function () {
    lbl.textContent = 'New Task';
    form.action     = '/todos/new';
    form.reset();
    descEl.value    = '';
    recurDaysEl.value = '';
    endEdit();
    var activeSectionId = getActiveSectionId();
    if (activeSectionId && sectionEl) { sectionEl.value = activeSectionId; }
  });

  modal.addEventListener('shown.bs.modal', function () {
    titleEl.focus();
    titleEl.select();
  });

  var modalLabelEl = document.getElementById('modalLabel');
  if (modalLabelEl) { initLabelPicker(modalLabelEl, PRESET_LABELS); }

  /* ── Subtask functions ─────────────────────────────────────────────────── */
  window.toggleSubtaskCollapse = function (subtaskId) {
    const row = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"]');
    if (!row) return;
    row.classList.toggle('subtask-collapsed');
    const isCollapsed = row.classList.contains('subtask-collapsed');
    try { localStorage.setItem('subtask_collapsed_' + subtaskId, isCollapsed ? '1' : '0'); } catch(e) {}
  };
  window.showAddChild = function (subtaskId) {
    const form = document.querySelector('.subtask-add-child-form[data-parent-id="' + subtaskId + '"]');
    if (form) { form.style.display = 'block'; var inp = form.querySelector('input[name="input"]'); if (inp) inp.focus(); }
  };
  window.hideAddChild = function (subtaskId) {
    const form = document.querySelector('.subtask-add-child-form[data-parent-id="' + subtaskId + '"]');
    if (form) form.style.display = 'none';
  };
  var activeSubtaskRow = null;
  function setActiveSubtaskRow(row) {
    if (activeSubtaskRow) activeSubtaskRow.classList.remove('subtask-row-active');
    activeSubtaskRow = row || null;
    if (activeSubtaskRow) activeSubtaskRow.classList.add('subtask-row-active');
  }
  window.setActiveSubtaskRow = setActiveSubtaskRow;
  document.addEventListener('click', function(e) {
    var tag = e.target.tagName;
    if (['INPUT','BUTTON','TEXTAREA','SELECT','A'].indexOf(tag) !== -1) return;
    if (e.target.closest('form')) return;
    var row = e.target.closest('.subtask-row[data-clickable-row]');
    if (!row) { setActiveSubtaskRow(null); return; }
    setActiveSubtaskRow(activeSubtaskRow === row ? null : row);
  });
  window.showSubtaskEdit = function (subtaskId) {
    var editForm = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-edit');
    var viewDiv = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-view');
    if (viewDiv) viewDiv.style.display = 'none';
    if (editForm) {
      editForm.style.display = 'block';
      var inp = editForm.querySelector('input[name="title"]'); if (inp) inp.focus();
    }
  };
  window.hideSubtaskEdit = function (subtaskId) {
    var editForm = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-edit');
    var viewDiv = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-view');
    if (editForm) editForm.style.display = 'none';
    if (viewDiv) viewDiv.style.display = '';
  };
  function initSubtaskSortables() {
    document.querySelectorAll('.subtask-list-root, .subtask-children').forEach(function(container) {
      if (container._sortable) return;
      if (typeof Sortable === 'undefined') return;
      container._sortable = new Sortable(container, {
        handle: '.drag-handle-sub',
        animation: 150,
        delay: 150,
        delayOnTouchOnly: true,
        onEnd: function() {
          var taskId = container.closest('[data-task-id]');
          taskId = taskId ? taskId.dataset.taskId : null;
          if (!taskId) return;
          var ids = Array.from(container.querySelectorAll(':scope > .subtask-row')).map(function(el) { return el.dataset.subtaskId; });
          var parentId = container.dataset.parentId || '';
          fetch('/todos/' + taskId + '/subtasks/reorder', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ids: ids, parent_subtask_id: parentId})
          });
        }
      });
    });
  }
  document.querySelectorAll('.subtask-row[data-subtask-id]').forEach(function(row) {
    const id = row.dataset.subtaskId;
    try { if (localStorage.getItem('subtask_collapsed_' + id) === '1') row.classList.add('subtask-collapsed'); } catch(e) {}
  });
  initSubtaskSortables();
  document.addEventListener('htmx:afterSettle', initSubtaskSortables);
`

// buildPoliciesBannerScript builds the inline script for the policies dismiss
// banner. Uses Raw injection so Go data can be embedded in the script safely.
func buildPoliciesBannerScript(policies []models.Policy) string {
	var sb strings.Builder
	sb.WriteString("<script>(function(){var POLICIES=[")
	for i, p := range policies {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{id:"%s",reappear:%d}`,
			strings.ReplaceAll(p.ID.String(), `"`, `\"`),
			p.ReappearAfterHours,
		)
	}
	sb.WriteString(
		`];var ids=POLICIES.map(function(p){return p.id;}).sort().join(',');`,
	)
	sb.WriteString(
		`var minReappear=POLICIES.reduce(function(m,p){return Math.min(m,p.reappear);},Infinity);`,
	)
	sb.WriteString(`var storageKey='policies:'+ids;`)
	sb.WriteString(`var stored=localStorage.getItem(storageKey);`)
	sb.WriteString(`var banner=document.getElementById('policiesBanner');`)
	sb.WriteString(`var dismissed=false;`)
	sb.WriteString(
		`if(stored){if(minReappear===0){dismissed=true;}else{var elapsed=(Date.now()-parseInt(stored,10))/3600000;if(elapsed<minReappear){dismissed=true;}}}`,
	)
	sb.WriteString(
		`if(dismissed){banner.classList.add('d-none');}else{document.getElementById('policiesDismiss').addEventListener('click',function(){localStorage.setItem(storageKey,String(Date.now()));banner.classList.add('d-none');});}`,
	)
	sb.WriteString("}());</script>")
	return sb.String()
}

// buildFormPageInitScript builds the JS init vars for the form/edit page.
func buildFormPageInitScript(d FormPageData) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "var isEdit=%v;", d.IsEdit)
	sb.WriteString("var PRESET_LABELS=[")
	for i, p := range d.Presets.Labels {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('"')
		sb.WriteString(strings.ReplaceAll(p.Value, `"`, `\"`))
		sb.WriteByte('"')
	}
	sb.WriteString("];")
	return sb.String()
}

// formPageStaticJS is the static JS body for the form/edit page.
const formPageStaticJS = `
  /* ── Label autocomplete dropdown ──────────────────────────────────────── */
  var labelAcDrop = null;
  var labelAcOwner = null;

  function initLabelAc(inputEl, presets) {
    inputEl.addEventListener('input', function () {
      var val = inputEl.value;
      var lastCommaIdx = val.lastIndexOf(',');
      var segment = lastCommaIdx >= 0 ? val.slice(lastCommaIdx + 1).trim() : val.trim();
      if (!segment) { if (labelAcDrop) { labelAcDrop.style.display = 'none'; } return; }
      var matches = presets.filter(function (p) { return p.toLowerCase().startsWith(segment.toLowerCase()); });
      if (!labelAcDrop) {
        labelAcDrop = document.createElement('ul');
        labelAcDrop.className = 'list-group shadow-sm';
        labelAcDrop.style.cssText = 'position:fixed;z-index:9999;display:none;max-height:200px;overflow-y:auto';
        document.body.appendChild(labelAcDrop);
      }
      labelAcOwner = inputEl;
      labelAcDrop.innerHTML = '';
      matches.forEach(function (val) {
        var li = document.createElement('li');
        li.className = 'list-group-item';
        li.style.cssText = 'cursor:pointer;padding:.35rem .75rem;font-size:.875rem';
        li.textContent = val;
        li.addEventListener('mousedown', function (e) {
          e.preventDefault();
          var currentVal = inputEl.value;
          var lastIdx = currentVal.lastIndexOf(',');
          if (lastIdx >= 0) { inputEl.value = currentVal.slice(0, lastIdx + 1) + ' ' + val; } else { inputEl.value = val; }
          if (labelAcDrop) { labelAcDrop.style.display = 'none'; }
        });
        labelAcDrop.appendChild(li);
      });
      if (matches.length) {
        var rect = inputEl.getBoundingClientRect();
        labelAcDrop.style.top = (rect.bottom + 2) + 'px';
        labelAcDrop.style.left = rect.left + 'px';
        labelAcDrop.style.width = Math.max(rect.width, 160) + 'px';
        labelAcDrop.style.display = '';
      } else { if (labelAcDrop) { labelAcDrop.style.display = 'none'; } }
    });
    inputEl.addEventListener('blur', function () { setTimeout(function () { if (labelAcDrop) { labelAcDrop.style.display = 'none'; } }, 150); });
    inputEl.addEventListener('keydown', function (e) { if (e.key === 'Escape' && labelAcDrop) { labelAcDrop.style.display = 'none'; } });
  }

  function initLabelPicker(inputEl, presets) {
    if (inputEl._labelPickerBound) { return; }
    inputEl._labelPickerBound = true;
    var selected = inputEl.value ? inputEl.value.split(',').map(function(s){ return s.trim(); }).filter(Boolean) : [];
    inputEl.style.display = 'none';
    var wrap = document.createElement('div');
    wrap.className = 'lp-wrap';
    if (inputEl.classList.contains('view-pill-input')) { wrap.classList.add('lp-pill'); }
    var trigger = document.createElement('div');
    trigger.className = 'lp-trigger';
    var searchInput = document.createElement('input');
    searchInput.type = 'text'; searchInput.className = 'lp-search'; searchInput.autocomplete = 'off';
    searchInput.placeholder = selected.length ? '' : (inputEl.placeholder || 'Labels…');
    trigger.appendChild(searchInput);
    var drop = document.createElement('div');
    drop.className = 'lp-drop'; drop.style.display = 'none';
    document.body.appendChild(drop);
    inputEl.parentNode.insertBefore(wrap, inputEl);
    wrap.appendChild(trigger);
    function syncInput() {
      inputEl.value = selected.join(', ');
      inputEl.dispatchEvent(new Event('input', {bubbles: true}));
      inputEl.dispatchEvent(new Event('change', {bubbles: true}));
    }
    function isSelected(label) { return selected.some(function(s) { return s.toLowerCase() === label.toLowerCase(); }); }
    function renderTrigger() {
      Array.from(trigger.querySelectorAll('.lp-badge')).forEach(function(b) { trigger.removeChild(b); });
      selected.forEach(function(label) {
        var badge = document.createElement('span'); badge.className = 'lp-badge';
        var txt = document.createTextNode(label + ' '); badge.appendChild(txt);
        var x = document.createElement('button'); x.type = 'button'; x.className = 'lp-badge-x'; x.textContent = '×';
        x.addEventListener('mousedown', function(e) { e.preventDefault(); removeLabel(label); });
        badge.appendChild(x); trigger.insertBefore(badge, searchInput);
      });
      searchInput.placeholder = selected.length ? '' : (inputEl.placeholder || 'Labels…');
    }
    function addLabel(label) {
      label = label.trim(); if (!label || isSelected(label)) { return; }
      var canonical = presets.find(function(p) { return p.toLowerCase() === label.toLowerCase(); });
      selected.push(canonical || label); renderTrigger(); syncInput(); searchInput.value = ''; renderDrop();
    }
    function removeLabel(label) {
      selected = selected.filter(function(s) { return s.toLowerCase() !== label.toLowerCase(); });
      renderTrigger(); syncInput(); renderDrop();
    }
    function renderDrop() {
      var filter = searchInput.value.trim().toLowerCase();
      var filtered = filter ? presets.filter(function(p) { return p.toLowerCase().indexOf(filter) !== -1; }) : presets;
      drop.innerHTML = '';
      filtered.forEach(function(label) {
        var sel = isSelected(label); var item = document.createElement('div');
        item.className = 'lp-item' + (sel ? ' lp-item-selected' : '');
        var chk = document.createElement('span'); chk.className = 'lp-chk'; chk.textContent = sel ? '☑' : '☐'; item.appendChild(chk);
        var lbl = document.createElement('span'); lbl.textContent = label; item.appendChild(lbl);
        item.addEventListener('mousedown', function(e) { e.preventDefault(); if (isSelected(label)) { removeLabel(label); } else { addLabel(label); } searchInput.focus(); });
        drop.appendChild(item);
      });
      var typedVal = searchInput.value.trim();
      if (typedVal && !isSelected(typedVal) && !presets.some(function(p) { return p.toLowerCase() === typedVal.toLowerCase(); })) {
        var addItem = document.createElement('div'); addItem.className = 'lp-item lp-add';
        addItem.textContent = '+ Add "' + typedVal + '"';
        addItem.addEventListener('mousedown', function(e) { e.preventDefault(); addLabel(typedVal); searchInput.focus(); });
        drop.appendChild(addItem);
      }
      var rect = trigger.getBoundingClientRect();
      drop.style.top = (rect.bottom + 2) + 'px'; drop.style.left = rect.left + 'px';
      drop.style.minWidth = Math.max(rect.width, 180) + 'px';
    }
    function openDrop() { renderDrop(); drop.style.display = ''; }
    function closeDrop() { drop.style.display = 'none'; }
    searchInput.addEventListener('focus', openDrop);
    searchInput.addEventListener('input', renderDrop);
    searchInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') { e.preventDefault(); var v = searchInput.value.trim(); if (v) { addLabel(v); } }
      else if (e.key === 'Escape') { closeDrop(); }
      else if (e.key === 'Backspace' && !searchInput.value && selected.length) { removeLabel(selected[selected.length - 1]); }
    });
    searchInput.addEventListener('blur', function() { setTimeout(closeDrop, 150); });
    trigger.addEventListener('mousedown', function(e) { if (e.target !== searchInput) { e.preventDefault(); searchInput.focus(); } });
    function outsideClick(e) {
      var path = e.composedPath ? e.composedPath() : (e.path || []);
      var inside = path.length ? (path.indexOf(drop) !== -1 || path.indexOf(wrap) !== -1) : (wrap.contains(e.target) || drop.contains(e.target));
      if (!inside) { closeDrop(); }
    }
    document.addEventListener('mousedown', outsideClick);
    renderTrigger();
  }

  function renderMarkdown(src) { return src.trim() ? marked.parse(src) : ''; }

  var descTextarea = document.getElementById('formDesc');
  var descPreview  = document.getElementById('formDescPreview');

  function syncPreview() {
    if (!descTextarea || !descPreview) { return; }
    var html = renderMarkdown(descTextarea.value);
    if (html) { descPreview.innerHTML = html; }
    else { descPreview.innerHTML = '<span class="text-muted small">Preview will appear here…</span>'; }
  }

  if (descTextarea) {
    descTextarea.addEventListener('input', function () {
      this.style.height = 'auto';
      this.style.height = Math.max(220, this.scrollHeight) + 'px';
      syncPreview();
    });
    descTextarea.style.height = Math.max(220, descTextarea.scrollHeight) + 'px';
    syncPreview();
  }

  if (isEdit) {
    var descViewWrap  = document.getElementById('descViewWrap');
    var descView      = document.getElementById('descView');
    var descEditWrap  = document.getElementById('descEditWrap');

    function renderDescView() {
      if (!descTextarea || !descView) { return; }
      descView.innerHTML = renderMarkdown(descTextarea.value);
    }
    renderDescView();

    window.activateDesc = function () {
      if (!descViewWrap || !descEditWrap) { return; }
      descViewWrap.style.display = 'none'; descEditWrap.style.display = '';
      if (descTextarea) { descTextarea.focus(); }
    };
    window.deactivateDesc = function () {
      if (!descViewWrap || !descEditWrap) { return; }
      renderDescView(); descEditWrap.style.display = 'none'; descViewWrap.style.display = '';
    };

    var mainLabelInput = document.querySelector('input[name="label"].view-pill-input');
    if (mainLabelInput) { mainLabelInput.removeAttribute('list'); initLabelPicker(mainLabelInput, PRESET_LABELS); }

    var autoSaveTimeout = null;
    function autoSave() {
      var editForm = document.getElementById('task-edit-form');
      if (!editForm) { return; }
      var saveStatus = document.getElementById('saveStatus');
      if (saveStatus) { saveStatus.textContent = 'Saving…'; }
      var params = new URLSearchParams(new FormData(editForm));
      fetch(editForm.action, { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded', 'X-Auto-Save': '1'}, body: params.toString() })
        .then(function () { if (saveStatus) { saveStatus.textContent = 'Saved'; setTimeout(function () { if (saveStatus) { saveStatus.textContent = ''; } }, 2000); } })
        .catch(function () { if (saveStatus) { saveStatus.textContent = 'Save failed'; } });
    }
    function autoSaveDebounced() { clearTimeout(autoSaveTimeout); autoSaveTimeout = setTimeout(autoSave, 1500); }

    var titleInput = document.querySelector('#task-edit-form input[name="title"]');
    if (titleInput) { titleInput.addEventListener('input', autoSaveDebounced); }
    var labelInput = document.querySelector('#task-edit-form input[name="label"]');
    if (labelInput) { labelInput.addEventListener('input', autoSaveDebounced); }
    var dueDateInput = document.querySelector('#task-edit-form input[name="due_date"]');
    if (dueDateInput) { dueDateInput.addEventListener('input', autoSaveDebounced); }
    var deadlineInput = document.querySelector('#task-edit-form input[name="deadline"]');
    if (deadlineInput) { deadlineInput.addEventListener('input', autoSaveDebounced); }
    var descInput = document.getElementById('formDesc');
    if (descInput) { descInput.addEventListener('input', autoSaveDebounced); }
    var prioritySelect = document.querySelector('#task-edit-form select[name="priority"]');
    if (prioritySelect) { prioritySelect.addEventListener('change', autoSaveDebounced); }
    var sectionSelect = document.querySelector('#task-edit-form select[name="section_id"]');
    if (sectionSelect) { sectionSelect.addEventListener('change', autoSaveDebounced); }

    var submitButtons = document.querySelectorAll('#task-edit-form [type="submit"], [form="task-edit-form"]');
    submitButtons.forEach(function (btn) {
      btn.addEventListener('click', function (event) { event.preventDefault(); autoSave(); });
    });
  }

  window.showSubtaskEdit = function (subtaskId) {
    var editForm = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-edit');
    var viewDiv = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-view');
    if (viewDiv) viewDiv.style.display = 'none';
    if (editForm) { editForm.style.display = 'block'; var inp = editForm.querySelector('input[name="title"]'); if (inp) inp.focus(); }
  };
  window.hideSubtaskEdit = function (subtaskId) {
    var editForm = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-edit');
    var viewDiv = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"] .subtask-title-view');
    if (editForm) editForm.style.display = 'none';
    if (viewDiv) viewDiv.style.display = '';
  };
  window.toggleSubtaskCollapse = function (subtaskId) {
    const row = document.querySelector('.subtask-row[data-subtask-id="' + subtaskId + '"]');
    if (!row) return;
    row.classList.toggle('subtask-collapsed');
    const isCollapsed = row.classList.contains('subtask-collapsed');
    try { localStorage.setItem('subtask_collapsed_' + subtaskId, isCollapsed ? '1' : '0'); } catch(e) {}
  };
  window.showAddChild = function (subtaskId) {
    const form = document.querySelector('.subtask-add-child-form[data-parent-id="' + subtaskId + '"]');
    if (form) { form.style.display = 'block'; var inp = form.querySelector('input[name="input"]'); if (inp) inp.focus(); }
  };
  window.hideAddChild = function (subtaskId) {
    const form = document.querySelector('.subtask-add-child-form[data-parent-id="' + subtaskId + '"]');
    if (form) form.style.display = 'none';
  };

  var addLinkBtn = document.getElementById('addLink');
  if (addLinkBtn) {
    addLinkBtn.addEventListener('click', function () {
      var row = document.createElement('div');
      row.className = 'row g-2 mb-2 link-row';
      row.innerHTML = '<div class="col-7"><input type="url" name="link_url" class="form-control form-control-sm" placeholder="https://…"></div>'
        + '<div class="col-4"><input type="text" name="link_label" class="form-control form-control-sm" placeholder="Label (e.g. PR #42)"></div>'
        + '<div class="col-1"><button type="button" class="btn btn-outline-danger btn-sm w-100" onclick="this.closest(\'.link-row\').remove()">×</button></div>';
      document.getElementById('links').appendChild(row);
    });
  }

  if (!isEdit) {
    document.querySelectorAll('input[name="label"]').forEach(function(el) { initLabelPicker(el, PRESET_LABELS); });
  }

  function initSubtaskSortables() {
    document.querySelectorAll('.subtask-list-root, .subtask-children').forEach(function(container) {
      if (container._sortable) return;
      if (typeof Sortable === 'undefined') return;
      container._sortable = new Sortable(container, {
        handle: '.drag-handle-sub', animation: 150, delay: 150, delayOnTouchOnly: true,
        onEnd: function() {
          var taskId = container.closest('[data-task-id]'); taskId = taskId ? taskId.dataset.taskId : null;
          if (!taskId) return;
          var ids = Array.from(container.querySelectorAll(':scope > .subtask-row')).map(function(el) { return el.dataset.subtaskId; });
          var parentId = container.dataset.parentId || '';
          fetch('/todos/' + taskId + '/subtasks/reorder', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({ids: ids, parent_subtask_id: parentId}) });
        }
      });
    });
  }

  document.querySelectorAll('.subtask-row[data-subtask-id]').forEach(function(row) {
    const id = row.dataset.subtaskId;
    try { if (localStorage.getItem('subtask_collapsed_' + id) === '1') row.classList.add('subtask-collapsed'); } catch(e) {}
  });
  initSubtaskSortables();
  document.addEventListener('htmx:afterSettle', initSubtaskSortables);
`
