#!/usr/bin/env python3
"""
A stand-in for a 7 Days to Die server that actually renders its map.

The panel's own test server has EnableMapRendering off in serverconfig.xml, and
that can only be changed with a server restart, so there is no way to see the
map page working against it. This answers the handful of endpoints the map
needs, with tiles generated on the fly, so the viewer can be checked end to
end: the projection, the tile row flip, the caching headers and the overlays.

Not part of the build. Run it by hand:

    python3 scripts/fake-game-server.py 8099
"""

import json
import struct
import sys
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TILE_SIZE = 128
MAX_ZOOM = 4
WORLD = 6144

# Tiles only exist where somebody has explored. A band across the middle of the
# world is "explored" so that the edge between drawn and blank is visible, and
# so a wrong row flip shows up as the band being in the wrong place.
EXPLORED_ROWS = range(-6, 3)


def envelope(data):
    return json.dumps({"data": data, "meta": {"serverTime": "2026-09-13T00:00:00Z"}}).encode()


def png(width, height, pixels):
    """Encode raw RGB rows as a PNG, so this has no image library to install."""
    raw = b"".join(b"\x00" + row for row in pixels)

    def chunk(tag, payload):
        body = tag + payload
        return struct.pack(">I", len(payload)) + body + struct.pack(">I", zlib.crc32(body))

    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(raw, 6))
        + chunk(b"IEND", b"")
    )


def tile_image(z, x, y):
    """
    A tile whose colour encodes its own coordinates.

    Reading the colours off the screen is how the projection gets checked: if x
    and y were swapped or a row flipped, the gradient runs the wrong way and it
    is obvious rather than subtle.
    """
    red = (x * 37) % 200 + 30
    green = (y * 53) % 200 + 30
    blue = (z * 50) % 200 + 30

    rows = []
    for row in range(TILE_SIZE):
        pixels = bytearray()
        for col in range(TILE_SIZE):
            # A grid line on the tile's own edges, so tile seams are visible.
            edge = row < 2 or col < 2
            if edge:
                pixels += bytes((240, 240, 240))
            else:
                pixels += bytes((red, green, blue))
        rows.append(bytes(pixels))
    return png(TILE_SIZE, TILE_SIZE, rows)


PLAYERS = [
    {"entityId": 171, "name": "Ana", "position": {"x": 300, "y": 61, "z": 200},
     "platformId": {"combinedString": "Steam_76561198000000001"},
     "crossplatformId": {"combinedString": "EOS_0001"},
     "level": 12, "health": 100, "stamina": 90.0, "score": 4, "deaths": 1,
     "kills": {"zombies": 30, "players": 0},
     "banned": {"banActive": False, "reason": None, "until": None},
     "ip": "10.0.0.5", "ping": 24},
    {"entityId": 172, "name": "Bo", "position": {"x": -500, "y": 40, "z": -250},
     "platformId": {"combinedString": "Steam_76561198000000002"},
     "crossplatformId": {"combinedString": "EOS_0002"},
     "level": 8, "health": 72, "stamina": 44.0, "score": 2, "deaths": 3,
     "kills": {"zombies": 11, "players": 0},
     "banned": {"banActive": False, "reason": None, "until": None},
     "ip": "10.0.0.6", "ping": 31},
]

# Enough zombies to show that hundreds of markers stay smooth.
HOSTILES = [
    {"id": 1000 + n, "name": "zombieBiker",
     "position": {"x": 250 + (n % 30) * 14, "y": 61, "z": 150 + (n // 30) * 14}}
    for n in range(300)
]

ANIMALS = [
    {"id": 2000 + n, "name": "animalStag",
     "position": {"x": -400 + n * 25, "y": 55, "z": 300 + (n % 5) * 30}}
    for n in range(40)
]

LAND_CLAIMS = {
    "claimsize": 41,
    "claimowners": [
        {"steamid": "Steam_76561198000000001", "playername": "Ana", "claimactive": True,
         "claims": [{"x": 300, "y": 61, "z": 200}, {"x": 380, "y": 61, "z": 240}]},
        {"steamid": "Steam_76561198000000002", "playername": "Bo", "claimactive": False,
         "claims": [{"x": -500, "y": 40, "z": -250}]},
    ],
}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args):
        pass

    def send(self, status, body=b"", content_type="application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        self.send(200, envelope({"command": "", "parameters": "", "result": ""}))

    def do_GET(self):
        path = self.path.split("?")[0]

        if path.startswith("/map/"):
            return self.serve_tile(path)

        routes = {
            "/api/map/config": lambda: envelope({
                "enabled": True, "mapBlockSize": TILE_SIZE,
                "maxZoom": MAX_ZOOM, "mapSize": {"x": WORLD, "y": 255, "z": WORLD},
            }),
            "/api/player": lambda: envelope({"players": PLAYERS}),
            "/api/hostile": lambda: envelope(HOSTILES),
            "/api/animal": lambda: envelope(ANIMALS),
            "/api/serverstats": lambda: envelope({
                "gameTime": {"days": 7, "hours": 21, "minutes": 30},
                "players": len(PLAYERS), "hostiles": len(HOSTILES), "animals": len(ANIMALS),
            }),
            "/api/serverinfo": lambda: envelope([
                {"name": "GameHost", "value": "Fixture", "type": "String"},
                {"name": "ServerVersion", "value": "V 2.0 (b1)", "type": "String"},
                {"name": "CurrentPlayers", "value": len(PLAYERS), "type": "Integer"},
                {"name": "MaxPlayers", "value": 8, "type": "Integer"},
                {"name": "GameWorld", "value": "Navezgane", "type": "String"},
                {"name": "DayNightLength", "value": 60, "type": "Integer"},
                {"name": "DayLightLength", "value": 18, "type": "Integer"},
            ]),
            "/api/log": lambda: envelope({"entries": [], "firstLine": 0, "lastLine": 0}),
            "/api/gamestats": lambda: envelope([]),
            "/api/gameprefs": lambda: envelope([]),
            "/api/command": lambda: envelope({"commands": []}),
            "/api/item": lambda: envelope([]),
            "/api/entityclass": lambda: envelope([]),
        }

        if path == "/api/getlandclaims":
            return self.send(200, json.dumps(LAND_CLAIMS).encode())
        if path == "/api/getplayerlist":
            return self.send(200, json.dumps({
                "total": len(PLAYERS), "firstResult": 0,
                "players": [{
                    "steamid": p["platformId"]["combinedString"],
                    "crossplatformid": p["crossplatformId"]["combinedString"],
                    "entityid": p["entityId"], "ip": p["ip"], "name": p["name"],
                    "online": True, "position": p["position"],
                    "totalplaytime": 3600, "lastonline": None,
                    "ping": p["ping"], "banned": False,
                } for p in PLAYERS],
            }).encode())

        if path in routes:
            return self.send(200, routes[path]())
        self.send(404, b"{}")

    def serve_tile(self, path):
        try:
            z, x, rest = path[len("/map/"):].split("/")
            y = int(rest.removesuffix(".png"))
            z, x = int(z), int(x)
        except ValueError:
            return self.send(404, b"")

        span = (WORLD // TILE_SIZE) // 2
        in_world = -span <= x < span and -span <= y < span
        if not in_world or y not in EXPLORED_ROWS:
            # The renderer has never drawn here, which is how the real server
            # answers for ground nobody has walked over.
            return self.send(404, b"")
        self.send(200, tile_image(z, x, y), "image/png")


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8099
    print(f"fake game server on http://127.0.0.1:{port}")
    ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
