// Portal htmx hardening. Loaded immediately after htmx.min.js and before
// DOMContentLoaded, so these settings apply to htmx's initial process pass.
// A strict CSP forbids inline <script>, so this configuration lives in its
// own file rather than an inline tag or hx-config meta attribute.
htmx.config.allowEval = false;             // no eval-based features
htmx.config.includeIndicatorStyles = false; // do not inject an inline <style> (CSP blocks it); .htmx-indicator lives in style.css
htmx.config.historyEnabled = false;         // no localStorage page snapshots of sensitive usage/cost pages
htmx.config.selfRequestsOnly = true;        // default in 2.x; asserted explicitly
