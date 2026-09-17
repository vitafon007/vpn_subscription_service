(function () {
  "use strict";

  var HEARTBEAT_MS = 30000;

  function postJSON(url, body) {
    return fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
      keepalive: true,
    });
  }

  function track(type, payload) {
    return postJSON("/invite/api/events", {
      type: type,
      payload: payload || {},
    }).catch(function () {});
  }

  function startHeartbeat(variant) {
    var visible = document.visibilityState !== "hidden";
    function beat() {
      if (!visible) return;
      track("heartbeat", { variant: variant || "unknown" });
    }
    beat();
    setInterval(beat, HEARTBEAT_MS);
    document.addEventListener("visibilitychange", function () {
      if (document.visibilityState === "hidden") {
        visible = false;
        track("visibility_hidden", { variant: variant });
      } else {
        visible = true;
        track("visibility_visible", { variant: variant });
        beat();
      }
    });
  }

  function syncCountdown(revealUnix, serverUnix) {
    var skew = Date.now() - serverUnix * 1000;
    var ended = false;

    function pad(n) {
      return n < 10 ? "0" + n : String(n);
    }

    function tick() {
      var now = Date.now() - skew;
      var left = Math.max(0, revealUnix * 1000 - now);
      var total = Math.floor(left / 1000);
      var days = Math.floor(total / 86400);
      var hours = Math.floor((total % 86400) / 3600);
      var mins = Math.floor((total % 3600) / 60);
      var secs = total % 60;

      var d = document.getElementById("cd-days");
      var h = document.getElementById("cd-hours");
      var m = document.getElementById("cd-mins");
      var s = document.getElementById("cd-secs");
      if (d) d.textContent = pad(days);
      if (h) h.textContent = pad(hours);
      if (m) m.textContent = pad(mins);
      if (s) s.textContent = pad(secs);

      if (left <= 0 && !ended) {
        ended = true;
        if (navigator.vibrate) {
          try {
            navigator.vibrate(40);
          } catch (e) {}
        }
        setTimeout(function () {
          location.reload();
        }, 700);
      }
    }

    tick();
    setInterval(tick, 250);
  }

  function setupChips(root, max) {
    if (!root) return;
    root.addEventListener("click", function (e) {
      var btn = e.target.closest(".chip");
      if (!btn || !root.contains(btn)) return;
      var active = root.querySelectorAll(".chip.active");
      if (btn.classList.contains("active")) {
        btn.classList.remove("active");
        return;
      }
      if (active.length >= max) {
        active[0].classList.remove("active");
      }
      btn.classList.add("active");
    });
  }

  function selectedChips(root) {
    if (!root) return [];
    return Array.prototype.map.call(root.querySelectorAll(".chip.active"), function (el) {
      return el.getAttribute("data-value");
    });
  }

  function selectedSingle(root) {
    var vals = selectedChips(root);
    return vals.length ? vals[0] : "";
  }

  function setupSingleSelect(root) {
    if (!root) return;
    root.addEventListener("click", function (e) {
      var btn = e.target.closest(".chip");
      if (!btn || !root.contains(btn)) return;
      Array.prototype.forEach.call(root.querySelectorAll(".chip"), function (el) {
        el.classList.remove("active");
      });
      btn.classList.add("active");
    });
  }

  function show(el) {
    if (el) el.classList.remove("hidden");
  }

  function hide(el) {
    if (el) el.classList.add("hidden");
  }

  window.InviteApp = {
    track: track,
    startHeartbeat: startHeartbeat,
    syncCountdown: syncCountdown,
    setupChips: setupChips,
    setupSingleSelect: setupSingleSelect,
    selectedChips: selectedChips,
    selectedSingle: selectedSingle,
    show: show,
    hide: hide,
    postJSON: postJSON,
  };
})();
