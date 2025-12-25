#!/bin/sh
set -e

case "${1:-}" in
  configure)
    if ! getent group binance-pivot-monitor >/dev/null 2>&1; then
      addgroup --system binance-pivot-monitor >/dev/null
    fi

    if ! getent passwd binance-pivot-monitor >/dev/null 2>&1; then
      adduser --system --ingroup binance-pivot-monitor --no-create-home \
        --home /var/lib/binance-pivot-monitor --shell /usr/sbin/nologin \
        binance-pivot-monitor >/dev/null
    fi

    mkdir -p /var/lib/binance-pivot-monitor
    chown -R binance-pivot-monitor:binance-pivot-monitor /var/lib/binance-pivot-monitor || true

    if command -v systemctl >/dev/null 2>&1; then
      systemctl daemon-reload >/dev/null 2>&1 || true
      systemctl enable --now binance-pivot-monitor.service >/dev/null 2>&1 || true
    fi
    ;;
esac

exit 0
