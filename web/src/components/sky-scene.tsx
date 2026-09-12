import { useEffect, useRef } from "react";
import * as THREE from "three";
import { Sky } from "three/examples/jsm/objects/Sky.js";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import type { SkyConditions } from "@/components/sky";

/**
 * The world outside, rendered.
 *
 * A gradient between two colours can say "it is darker at night". It cannot say
 * what half past six actually looks like — that the light goes long and red near
 * the horizon while the top of the sky is still deep, and that the red is the
 * same air that makes noon blue, seen edge-on. That is atmospheric scattering,
 * and it is the whole reason a sky is worth looking at. So the sky here is the
 * Preetham model, driven by the server's own clock and its DayLightLength.
 *
 * Everything under it is a place rather than a backdrop: hills that roll, a
 * forest standing on them, and a cabin down in the hollow with its windows lit
 * and smoke going up. The point of the cabin is not decoration. This is a panel
 * for a game about surviving the night, and the whole of it is a countdown — so
 * the one warm thing on the page should be the light someone is keeping on, and
 * it should still be burning when the sky behind it goes black.
 *
 * Loaded only when the Weather tab is opened, because three.js is large and
 * nothing else in the panel needs it. `SkyPanel` stays the fallback for
 * anything without WebGL.
 */

const FOV = 52;

/** How high the sun gets at midday, and how far it sinks at midnight. */
const NOON_ELEVATION = 62;
const MIDNIGHT_DEPTH = 20;

const GROUND = 900;

/*
  The cabin, the camera, and the line between them.

  Kept together because they are one decision: the clearing has to be cut along
  the sightline, the land has to be flattened under the cabin, and the camera
  has to be aimed at it. Scattering these through the file is how you end up
  with a tree parked in front of the only thing worth looking at.
*/
const CABIN = { x: -16, z: -150 };
const CAMERA = { x: 34, z: 70 };
const CLEARING = 92;

interface Props {
  conditions: SkyConditions;
  hour: number;
  dawnHour: number;
  daylightHours: number;
  hordeTonight?: boolean;
}

/** Where the sun is, in degrees above the horizon and around it. */
function sunAngles(hour: number, dawnHour: number, daylightHours: number) {
  const duskHour = dawnHour + daylightHours;
  const day = hour >= dawnHour && hour < duskHour;
  const through = day
    ? (hour - dawnHour) / daylightHours
    : ((hour < dawnHour ? hour + 24 : hour) - duskHour) / (24 - daylightHours);
  // Below the horizon after dusk, which is what gives the shader a real
  // twilight rather than us having to fake one.
  const elevation = day
    ? Math.sin(Math.PI * through) * NOON_ELEVATION
    : -Math.sin(Math.PI * through) * MIDNIGHT_DEPTH;
  // Rising on the left and setting on the right, the same way round as the dial
  // on the Time tab.
  return { elevation, azimuth: 236 - through * 118, day, through };
}

/**
 * Where the moon is.
 *
 * Not simply the sun's position mirrored through the centre of the earth, which
 * is what it was: that puts it behind the camera for most of the night, so on
 * the one night anybody cares about there was no moon in the frame at all. It
 * gets its own arc instead, crossing the same way the sun does — up at dusk on
 * the left, down at dawn on the right.
 */
function moonAngles(hour: number, dawnHour: number, daylightHours: number) {
  const { day, through } = sunAngles(hour, dawnHour, daylightHours);
  return {
    elevation: day ? -30 : 15 + Math.sin(Math.PI * through) * 14,
    azimuth: 209 - through * 58,
  };
}

/* -------------------------------------------------------------- terrain -- */

/** Deterministic hash noise, so the same valley comes back every time. */
function noise(x: number, y: number): number {
  const n = Math.sin(x * 127.1 + y * 311.7) * 43758.5453;
  return n - Math.floor(n);
}

function smoothNoise(x: number, y: number): number {
  const xi = Math.floor(x);
  const yi = Math.floor(y);
  const xf = x - xi;
  const yf = y - yi;
  const u = xf * xf * (3 - 2 * xf);
  const v = yf * yf * (3 - 2 * yf);
  const a = noise(xi, yi);
  const b = noise(xi + 1, yi);
  const c = noise(xi, yi + 1);
  const d = noise(xi + 1, yi + 1);
  return a * (1 - u) * (1 - v) + b * u * (1 - v) + c * (1 - u) * v + d * u * v;
}

/**
 * The shape of the land.
 *
 * Deliberately flattened near the camera and around the cabin: the hollow has
 * to be a hollow, or the cabin sits on a slope and the trees march through it.
 */
function heightAt(x: number, z: number): number {
  const ridge =
    smoothNoise(x * 0.004, z * 0.004) * 70 +
    smoothNoise(x * 0.011, z * 0.011) * 22 +
    smoothNoise(x * 0.03, z * 0.03) * 5;
  const toClearing = Math.hypot(x - CABIN.x, z - CABIN.z);
  const bowl = Math.min(1, Math.max(0, (toClearing - CLEARING) / 170));
  return ridge * (0.1 + bowl * 0.9);
}

/**
 * Whether a tree may stand here.
 *
 * Two cuts: the clearing the cabin sits in, and a corridor along the sightline
 * so nothing grows between the camera and it. Without the second one the
 * nearest tree in a fifteen-hundred-tree forest lands in front of the lens and
 * hides the whole point of the scene.
 */
function canGrow(x: number, z: number): boolean {
  if (Math.hypot(x - CABIN.x, z - CABIN.z) < CLEARING) return false;
  if (z > CABIN.z && z < CAMERA.z + 40) {
    const along = (z - CAMERA.z) / (CABIN.z - CAMERA.z);
    const sightline = CAMERA.x + (CABIN.x - CAMERA.x) * along;
    // Widening with distance, so the corridor is a wedge rather than a slot.
    if (Math.abs(x - sightline) < 34 + along * 46) return false;
  }
  return true;
}

export default function SkyScene({
  conditions,
  hour,
  dawnHour,
  daylightHours,
  hordeTonight,
}: Props) {
  const holder = useRef<HTMLDivElement>(null);
  const kit = useRef<{
    renderer: THREE.WebGLRenderer;
    scene: THREE.Scene;
    camera: THREE.PerspectiveCamera;
    sky: Sky;
    sunLight: THREE.DirectionalLight;
    moonLight: THREE.DirectionalLight;
    ambient: THREE.HemisphereLight;
    fireLight: THREE.PointLight;
    windows: THREE.MeshBasicMaterial;
    stars: THREE.Points;
    moon: THREE.Sprite;
    clouds: THREE.Sprite[];
    sunGlow: THREE.Sprite;
    haze: THREE.Sprite[];
    mist: THREE.Sprite[];
    smoke: THREE.Sprite[];
    chimneyTop: THREE.Vector3;
    rain: THREE.Points;
    snow: THREE.Points;
    ground: THREE.MeshLambertMaterial;
    foliage: THREE.MeshLambertMaterial;
    rainSpeed: number;
    snowSpeed: number;
    drift: number;
    fire: number;
  } | null>(null);

  // Built once. Rebuilding a WebGL context whenever the weather changed would
  // be slow and a good way to exhaust the browser's context limit.
  useEffect(() => {
    const mount = holder.current;
    if (!mount) return;

    const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: "low-power" });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    // The single biggest depth cue in the frame. Without it the trees do not
    // stand on the ground, they are stickers laid over it.
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    mount.appendChild(renderer.domElement);
    renderer.domElement.style.display = "block";
    renderer.domElement.style.width = "100%";
    renderer.domElement.style.height = "100%";

    const scene = new THREE.Scene();

    // Standing on the valley side, looking down into the hollow. Slightly off
    // centre, because dead-centre framing looks like a diagram.
    const camera = new THREE.PerspectiveCamera(FOV, 1, 0.5, 40_000);
    camera.position.set(CAMERA.x, heightAt(CAMERA.x, CAMERA.z) + 30, CAMERA.z);
    // Aimed a little left of the cabin, so it sits off centre and there is sky
    // above it rather than the frame being all ground.
    camera.lookAt(CABIN.x - 6, heightAt(CABIN.x, CABIN.z) + 26, CABIN.z);

    const sky = new Sky();
    sky.scale.setScalar(20_000);
    scene.add(sky);

    const sunLight = new THREE.DirectionalLight(0xff_ff_ff, 2);
    /*
      A shadow frustum tight around the clearing rather than the whole map.

      A directional light has no position to speak of, so the shadow camera has
      to be aimed by hand. Covering the full nine-hundred-unit ground would
      spread two thousand pixels of shadow map over it and give stair-stepped
      edges; this covers the part actually on screen.
    */
    sunLight.castShadow = true;
    sunLight.shadow.mapSize.set(2048, 2048);
    sunLight.shadow.camera.left = -320;
    sunLight.shadow.camera.right = 320;
    sunLight.shadow.camera.top = 320;
    sunLight.shadow.camera.bottom = -320;
    sunLight.shadow.camera.near = 1;
    sunLight.shadow.camera.far = 3000;
    sunLight.shadow.bias = -0.0006;
    sunLight.shadow.normalBias = 0.6;
    sunLight.target.position.set(CABIN.x, 0, CABIN.z);
    scene.add(sunLight, sunLight.target);
    const ambient = new THREE.HemisphereLight(0x9c_b4_c8, 0x2a_20_1a, 0.6);
    scene.add(ambient);

    // Moonlight. Without it the land simply vanishes after dusk, and a weather
    // panel that goes blank for a third of every day is not a weather panel.
    const moonLight = new THREE.DirectionalLight(0x8f_a8_cc, 0);
    scene.add(moonLight);

    /* ---- the land ---- */

    const groundTexture = groundPattern();
    const groundMaterial = new THREE.MeshLambertMaterial({
      color: 0x47_4c_34,
      map: groundTexture,
      vertexColors: true,
    });
    const groundGeometry = new THREE.PlaneGeometry(GROUND * 2, GROUND * 2, 220, 220);
    const position = groundGeometry.getAttribute("position") as THREE.BufferAttribute;
    // Large-scale colour drift on top of the small-scale texture, so the land
    // has patches rather than being one flat swatch stretched to the horizon.
    const groundColours = new Float32Array(position.count * 3);
    for (let i = 0; i < position.count; i++) {
      const x = position.getX(i);
      const y = position.getY(i);
      // Still in the plane's own space here, so its Y is the world's Z.
      const h = heightAt(x, -y);
      position.setZ(i, h);
      const patch = smoothNoise(x * 0.006, -y * 0.006);
      const dry = smoothNoise(x * 0.02 + 40, -y * 0.02) * 0.35;
      groundColours[i * 3] = 0.8 + patch * 0.5 + dry;
      groundColours[i * 3 + 1] = 0.85 + patch * 0.38 + dry * 0.6;
      groundColours[i * 3 + 2] = 0.72 + patch * 0.3;
    }
    groundGeometry.setAttribute("color", new THREE.BufferAttribute(groundColours, 3));
    groundGeometry.computeVertexNormals();
    const ground = new THREE.Mesh(groundGeometry, groundMaterial);
    ground.rotation.x = -Math.PI / 2;
    ground.receiveShadow = true;
    scene.add(ground);

    /* ---- the forest ---- */

    /*
      One tree, built once and stamped fifteen hundred times.

      Three stacked cones rather than one: a single cone is a traffic bollard,
      and the tiers are what make the eye read conifer. Trunk and foliage share
      a geometry and are told apart by vertex colour, so the whole forest is one
      draw call instead of two — and the instance colour on top of that tints
      each tree slightly differently, because a forest of one identical green is
      the flattest thing there is.
    */
    const parts: THREE.BufferGeometry[] = [];
    const paint = (geometry: THREE.BufferGeometry, r: number, g: number, b: number) => {
      const count = geometry.getAttribute("position").count;
      const colours = new Float32Array(count * 3);
      for (let i = 0; i < count; i++) {
        colours[i * 3] = r;
        colours[i * 3 + 1] = g;
        colours[i * 3 + 2] = b;
      }
      geometry.setAttribute("color", new THREE.BufferAttribute(colours, 3));
      return geometry;
    };

    const trunk = new THREE.CylinderGeometry(0.9, 1.5, 11, 6);
    trunk.translate(0, 5.5, 0);
    parts.push(paint(trunk, 0.29, 0.21, 0.15));
    for (const [radius, height, at] of [
      [7, 15, 13],
      [5.4, 13, 20.5],
      [3.6, 11, 27.5],
    ] as const) {
      const tier = new THREE.ConeGeometry(radius, height, 8);
      tier.translate(0, at, 0);
      parts.push(paint(tier, 0.23, 0.32, 0.21));
    }
    const treeGeometry = mergeGeometries(parts, false)!;

    const foliageMaterial = new THREE.MeshLambertMaterial({ vertexColors: true });
    const spots: { x: number; z: number; scale: number }[] = [];
    let seed = 1;
    const random = () => {
      seed = (seed * 1103515245 + 12345) % 2147483648;
      return seed / 2147483648;
    };
    for (let i = 0; i < 1800; i++) {
      const x = (random() - 0.5) * GROUND * 1.7;
      const z = -random() * GROUND * 1.2 + 90;
      if (!canGrow(x, z)) continue;
      spots.push({ x, z, scale: 0.6 + random() * 1 });
    }

    const trees = new THREE.InstancedMesh(treeGeometry, foliageMaterial, spots.length);
    trees.castShadow = true;
    trees.receiveShadow = true;
    const matrix = new THREE.Matrix4();
    const quaternion = new THREE.Quaternion();
    const scaleVector = new THREE.Vector3();
    const tint = new THREE.Color();
    spots.forEach((spot, i) => {
      quaternion.setFromAxisAngle(new THREE.Vector3(0, 1, 0), random() * Math.PI * 2);
      scaleVector.set(spot.scale * (0.85 + random() * 0.3), spot.scale, spot.scale);
      matrix.compose(
        new THREE.Vector3(spot.x, heightAt(spot.x, spot.z) - 1, spot.z),
        quaternion,
        scaleVector,
      );
      trees.setMatrixAt(i, matrix);
      const shade = 0.72 + random() * 0.5;
      tint.setRGB(shade * (0.9 + random() * 0.2), shade, shade * (0.85 + random() * 0.2));
      trees.setColorAt(i, tint);
    });
    scene.add(trees);

    /* ---- the cabin ---- */

    const cabin = new THREE.Group();
    const cabinX = CABIN.x;
    const cabinZ = CABIN.z;
    cabin.position.set(cabinX, heightAt(cabinX, cabinZ), cabinZ);
    cabin.rotation.y = 0.42;
    scene.add(cabin);

    const WALL_W = 34;
    const WALL_H = 19;
    const WALL_D = 26;
    const PITCH = 0.62;

    const timber = new THREE.MeshLambertMaterial({ color: 0x8a_63_44 });
    const walls = new THREE.Mesh(new THREE.BoxGeometry(WALL_W, WALL_H, WALL_D), timber);
    walls.position.y = WALL_H / 2;
    walls.castShadow = true;
    walls.receiveShadow = true;
    cabin.add(walls);

    /*
      A pitched roof, sized so the two slabs actually meet.

      Each covers half the span, which at this pitch means it has to be longer
      than that half by exactly one over the cosine — get that wrong and the
      slabs cross over the ridge and the cabin wears a bowtie.
    */
    const roofMaterial = new THREE.MeshLambertMaterial({ color: 0x3a_2c_24 });
    const half = WALL_W / 2;
    const slabLength = half / Math.cos(PITCH) + 3;
    for (const side of [-1, 1]) {
      const slab = new THREE.Mesh(new THREE.BoxGeometry(slabLength, 1.6, WALL_D + 5), roofMaterial);
      slab.position.set((side * half) / 2, WALL_H + (half / 2) * Math.tan(PITCH), 0);
      slab.rotation.z = -side * PITCH;
      slab.castShadow = true;
      slab.receiveShadow = true;
      cabin.add(slab);
    }

    // Gable ends, filling the triangle the roof leaves open.
    for (const side of [-1, 1]) {
      const gable = new THREE.Shape();
      gable.moveTo(-half, 0);
      gable.lineTo(half, 0);
      gable.lineTo(0, half * Math.tan(PITCH));
      const end = new THREE.Mesh(new THREE.ShapeGeometry(gable), timber);
      end.position.set(0, WALL_H, (side * WALL_D) / 2);
      end.rotation.y = side > 0 ? 0 : Math.PI;
      cabin.add(end);
    }

    // The whole point. Kept unlit by the scene so the glow is its own, not
    // something the sun has to grant it.
    const windowMaterial = new THREE.MeshBasicMaterial({ color: 0xff_b4_57 });
    for (const [x, z, ry] of [
      [-8.5, WALL_D / 2 + 0.2, 0],
      [8.5, WALL_D / 2 + 0.2, 0],
      [WALL_W / 2 + 0.2, 2, Math.PI / 2],
      [WALL_W / 2 + 0.2, -8, Math.PI / 2],
    ] as const) {
      const pane = new THREE.Mesh(new THREE.PlaneGeometry(7, 6), windowMaterial);
      pane.position.set(x, 10, z);
      pane.rotation.y = ry;
      cabin.add(pane);
    }

    // A doorway, dark against the lit windows so the cabin reads as lived in
    // rather than as a lantern.
    const door = new THREE.Mesh(
      new THREE.PlaneGeometry(6, 10),
      new THREE.MeshBasicMaterial({ color: 0x1a_10_0a }),
    );
    door.position.set(0, 5, WALL_D / 2 + 0.2);
    cabin.add(door);

    const chimney = new THREE.Mesh(new THREE.BoxGeometry(5, 20, 5), roofMaterial);
    chimney.castShadow = true;
    chimney.position.set(-11, WALL_H + 6, -6);
    cabin.add(chimney);

    // A fire out front, which is what actually lights the ground here.
    const fireLight = new THREE.PointLight(0xff_8a_30, 0, 200, 2);
    const fireX = cabinX + 30;
    const fireZ = cabinZ + 34;
    fireLight.position.set(fireX, heightAt(fireX, fireZ) + 6, fireZ);
    scene.add(fireLight);

    const blob = blobTexture();

    const embers = new THREE.Sprite(
      new THREE.SpriteMaterial({
        map: blob,
        color: 0xff_8a_30,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
      }),
    );
    embers.position.copy(fireLight.position);
    embers.scale.setScalar(22);
    scene.add(embers);

    /* ---- the air ---- */

    // Where the smoke starts, kept in one place so the loop that recycles a
    // puff puts it back exactly where it first came from.
    const chimneyTop = new THREE.Vector3(
      cabinX - 11,
      heightAt(cabinX, cabinZ) + WALL_H + 16,
      cabinZ - 6,
    );
    const smoke: THREE.Sprite[] = [];
    for (let i = 0; i < 7; i++) {
      const puff = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: blob, transparent: true, depthWrite: false, opacity: 0.2 }),
      );
      puff.position.copy(chimneyTop).setY(chimneyTop.y + i * 9);
      puff.scale.setScalar(9 + i * 3.5);
      smoke.push(puff);
      scene.add(puff);
    }

    const starPositions = new Float32Array(1200 * 3);
    for (let i = 0; i < 1200; i++) {
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(Math.random());
      const r = 6000;
      starPositions[i * 3] = r * Math.sin(phi) * Math.cos(theta);
      starPositions[i * 3 + 1] = r * Math.cos(phi);
      starPositions[i * 3 + 2] = r * Math.sin(phi) * Math.sin(theta);
    }
    const starGeometry = new THREE.BufferGeometry();
    starGeometry.setAttribute("position", new THREE.BufferAttribute(starPositions, 3));
    const stars = new THREE.Points(
      starGeometry,
      new THREE.PointsMaterial({
        size: 1.8,
        sizeAttenuation: false,
        transparent: true,
        fog: false,
      }),
    );
    scene.add(stars);

    const moon = new THREE.Sprite(
      new THREE.SpriteMaterial({ map: blob, transparent: true, depthWrite: false, fog: false }),
    );
    moon.scale.setScalar(420);
    scene.add(moon);

    /*
      Atmosphere, in three cheap pieces.

      A glow around the sun, so it reads as something burning rather than a
      disc; banks of haze lying in the middle distance, which is what actually
      separates one ridge of trees from the next; and mist pooling in the
      hollow. None of it is volumetric — that would cost a raymarch — but the
      eye reads depth from layers between it and the horizon, and these are
      layers.
    */
    const sunGlow = new THREE.Sprite(
      new THREE.SpriteMaterial({
        map: blob,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
        fog: false,
      }),
    );
    sunGlow.scale.setScalar(1600);
    scene.add(sunGlow);

    const haze: THREE.Sprite[] = [];
    for (let i = 0; i < 10; i++) {
      const bank = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: blob, transparent: true, depthWrite: false, fog: false }),
      );
      const distance = 220 + i * 95;
      bank.position.set((Math.random() - 0.5) * 500, heightAt(0, -distance) + 14, -distance);
      bank.scale.set(700 + i * 130, 60 + i * 14, 1);
      haze.push(bank);
      scene.add(bank);
    }

    const mist: THREE.Sprite[] = [];
    for (let i = 0; i < 6; i++) {
      const pool = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: blob, transparent: true, depthWrite: false }),
      );
      pool.position.set(
        CABIN.x + (Math.random() - 0.5) * 340,
        heightAt(CABIN.x, CABIN.z) + 6,
        CABIN.z + (Math.random() - 0.5) * 260,
      );
      pool.scale.set(260 + Math.random() * 160, 34 + Math.random() * 20, 1);
      mist.push(pool);
      scene.add(pool);
    }

    const clouds: THREE.Sprite[] = [];
    for (let i = 0; i < 16; i++) {
      const sprite = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: blob, transparent: true, depthWrite: false, fog: false }),
      );
      sprite.position.set(
        (Math.random() - 0.5) * 3000,
        280 + Math.random() * 260,
        -300 - Math.random() * 1600,
      );
      sprite.scale.set(500 + Math.random() * 700, 150 + Math.random() * 130, 1);
      clouds.push(sprite);
      scene.add(sprite);
    }

    /** A column of falling particles, recycled from the bottom to the top. */
    function fall(count: number, size: number) {
      const positions = new Float32Array(count * 3);
      for (let i = 0; i < count; i++) {
        positions[i * 3] = (Math.random() - 0.5) * 420;
        positions[i * 3 + 1] = Math.random() * 260;
        positions[i * 3 + 2] = 60 - Math.random() * 480;
      }
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
      const points = new THREE.Points(
        geometry,
        new THREE.PointsMaterial({
          map: blob,
          size,
          sizeAttenuation: true,
          transparent: true,
          depthWrite: false,
        }),
      );
      scene.add(points);
      return points;
    }

    const rain = fall(3000, 0.5);
    const snow = fall(2000, 1.1);

    kit.current = {
      renderer,
      scene,
      camera,
      sky,
      sunLight,
      moonLight,
      ambient,
      fireLight,
      windows: windowMaterial,
      stars,
      moon,
      clouds,
      sunGlow,
      haze,
      mist,
      smoke,
      chimneyTop,
      rain,
      snow,
      ground: groundMaterial,
      foliage: foliageMaterial,
      rainSpeed: 200,
      snowSpeed: 26,
      drift: 0,
      fire: 0,
    };

    const resize = () => {
      const { clientWidth, clientHeight } = mount;
      if (clientWidth === 0 || clientHeight === 0) return;
      renderer.setSize(clientWidth, clientHeight, false);
      camera.aspect = clientWidth / clientHeight;
      camera.updateProjectionMatrix();
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(mount);

    let frame = 0;
    let last = performance.now();
    const tick = (now: number) => {
      const delta = Math.min(0.05, (now - last) / 1000);
      last = now;
      const k = kit.current;
      if (k) {
        for (const [points, speed] of [
          [k.rain, k.rainSpeed],
          [k.snow, k.snowSpeed],
        ] as const) {
          if (!points.visible) continue;
          const p = points.geometry.getAttribute("position") as THREE.BufferAttribute;
          const array = p.array as Float32Array;
          for (let i = 0; i < array.length; i += 3) {
            array[i + 1] -= speed * delta;
            array[i] += k.drift * delta;
            if (array[i + 1] < 0) {
              array[i + 1] = 260;
              array[i] = (Math.random() - 0.5) * 420;
            }
            if (Math.abs(array[i]) > 220) array[i] = -Math.sign(array[i]) * 220;
          }
          p.needsUpdate = true;
        }

        for (const cloud of k.clouds) {
          cloud.position.x += k.drift * 0.3 * delta;
          if (cloud.position.x > 1600) cloud.position.x = -1600;
          if (cloud.position.x < -1600) cloud.position.x = 1600;
        }

        // Smoke climbs and thins, then starts again from the chimney.
        for (const puff of k.smoke) {
          puff.position.y += (7 + k.drift * -0.02) * delta;
          puff.position.x += k.drift * 0.06 * delta;
          puff.scale.addScalar(2.2 * delta);
          const material = puff.material as THREE.SpriteMaterial;
          material.opacity = Math.max(0, material.opacity - 0.035 * delta);
          if (material.opacity <= 0.002) {
            puff.position.copy(k.chimneyTop);
            puff.scale.setScalar(9);
            material.opacity = 0.22;
          }
        }

        // A fire is never steady, and a steady orange light reads as a lamp.
        k.fire += delta;
        const flicker = 0.75 + Math.sin(k.fire * 9.1) * 0.12 + Math.sin(k.fire * 23.7) * 0.08;
        k.fireLight.intensity = k.fireLight.userData.base * flicker;

        k.renderer.render(k.scene, k.camera);
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);

    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      blob.dispose();
      scene.traverse((object) => {
        if (
          object instanceof THREE.Mesh ||
          object instanceof THREE.Points ||
          object instanceof THREE.InstancedMesh
        ) {
          object.geometry.dispose();
          const material = object.material;
          if (Array.isArray(material)) material.forEach((m) => m.dispose());
          else material.dispose();
        }
      });
      renderer.dispose();
      renderer.domElement.remove();
      kit.current = null;
    };
  }, []);

  // Everything the world reports, pushed into the scene already built.
  useEffect(() => {
    const k = kit.current;
    if (!k) return;

    const { cloud, fog, rain: rainRate, snow: snowRate, wind } = conditions;

    const { elevation, azimuth, day } = sunAngles(hour, dawnHour, daylightHours);

    const uniforms = k.sky.material.uniforms;
    // Haze thickens the air, which is what fog and heavy cloud both do: the
    // horizon whitens and the colour overhead washes out.
    uniforms.turbidity.value = 2 + fog * 7 + cloud * 3;
    uniforms.rayleigh.value = Math.max(0.4, 2.2 - fog * 0.9);
    uniforms.mieCoefficient.value = 0.005 + fog * 0.012;
    uniforms.mieDirectionalG.value = 0.8;

    const phi = THREE.MathUtils.degToRad(90 - elevation);
    const theta = THREE.MathUtils.degToRad(azimuth);
    const sun = new THREE.Vector3().setFromSphericalCoords(1, phi, theta);
    uniforms.sunPosition.value.copy(sun);

    const up = Math.max(0, elevation) / NOON_ELEVATION;
    const night = Math.min(1, Math.max(0, -elevation / 10));
    /*
      How far over the whole scene has gone to the blood moon.

      On or off, with no ramp. It was tied to how far the sun had sunk, which
      meant that at exactly the hour the horde is due — the sun still sitting on
      the horizon — the effect was barely on, and with a server clock that does
      not advance while nobody is online it stayed barely on. The game does not
      ease into a blood moon either: it is ten o'clock, and then it is happening.
    */
    const bloody = hordeTonight && !day ? 1 : 0;
    // Opened up after dark, the way an eye is. Holding the daylight exposure
    // through the night left a black rectangle.
    k.renderer.toneMappingExposure = 0.055 + up * 0.075 + night * (hordeTonight ? 0.15 : 0.11);

    // The sun itself: low light is long light, and long light is orange.
    k.sunLight.position
      .copy(sun)
      .multiplyScalar(900)
      .add(new THREE.Vector3(CABIN.x, 0, CABIN.z));
    // Shadows cost real time and say nothing once the sun is near the horizon,
    // where they stretch to the edge of the frustum and break up.
    k.sunLight.castShadow = day && up > 0.12;
    const warmth = Math.max(0, 1 - up * 2.6);
    k.sunLight.color.setRGB(1, 0.93 - warmth * 0.36, 0.84 - warmth * 0.6);
    k.sunLight.intensity = day ? (5 + up * 24) * (1 - cloud * 0.5) : 0.9;

    // Fill, so the faces turned away from the sun are shaded rather than
    // silhouetted. Without it a sun in front of the camera made every tree and
    // the cabin itself a flat black cut-out.
    k.ambient.intensity = day ? 3.2 + up * 3.6 : 0.9 + night * 0.7;
    k.ambient.color.setRGB(day ? 0.62 : 0.2, day ? 0.71 : 0.24, day ? 0.82 : 0.36);

    // The cabin keeps its lights on all day, but they only read as warmth once
    // the sky stops competing with them.
    k.windows.color.setRGB(1, 0.7 - night * 0.06, 0.34);
    k.fireLight.userData.base = 900 + night * 3400;
    k.fireLight.distance = 220 + night * 120;

    const starMaterial = k.stars.material as THREE.PointsMaterial;
    starMaterial.opacity = night * (1 - cloud * 0.85) * 0.85 * (1 - bloody * 0.9);
    k.stars.visible = starMaterial.opacity > 0.01;

    /*
      The blood moon, which is the only night this game is really about.

      Not a red moon on an otherwise ordinary night: the whole scene goes over.
      It comes up far bigger than it has any business being, the light it throws
      is red rather than blue so every tree is rimmed in it, the haze and the
      distance take the same colour, and the stars go out — a moon that bright
      washes them out, and losing them makes the sky feel closed in. The cabin
      is deliberately left alone. Its windows and its fire stay the warm yellow
      they always are, because the point of the whole panel is the contrast
      between the thing you are keeping lit and the thing coming for it.
    */
    const lunar = moonAngles(hour, dawnHour, daylightHours);
    const moonDirection = new THREE.Vector3().setFromSphericalCoords(
      1,
      THREE.MathUtils.degToRad(90 - lunar.elevation),
      THREE.MathUtils.degToRad(lunar.azimuth),
    );
    k.moon.position.copy(moonDirection).multiplyScalar(4000);
    k.moonLight.position
      .copy(moonDirection)
      .multiplyScalar(900)
      .add(new THREE.Vector3(CABIN.x, 0, CABIN.z));
    k.moonLight.intensity = night * (2.6 + bloody * 3.4);
    k.moonLight.color.lerpColors(new THREE.Color(0x8f_a8_cc), new THREE.Color(0xff_4a_32), bloody);
    const moonMaterial = k.moon.material as THREE.SpriteMaterial;
    moonMaterial.color.lerpColors(new THREE.Color(0xff_f1_dd), new THREE.Color(0xff_2a_1c), bloody);
    moonMaterial.opacity = day ? 0 : (1 - cloud * 0.7) * (0.9 + bloody * 0.1);
    // Additive on the horde night, so it burns through the haze in front of it
    // instead of being averaged away by it.
    moonMaterial.blending = bloody > 0 ? THREE.AdditiveBlending : THREE.NormalBlending;
    k.moon.visible = !day;
    k.moon.scale.setScalar(420 + bloody * 900);

    // The sun as a source rather than a dot: bigger and fiercer near the
    // horizon, where the light has the most air to travel through.
    k.sunGlow.position.copy(sun).multiplyScalar(2600);
    const glowMaterial = k.sunGlow.material as THREE.SpriteMaterial;
    const low = day ? Math.max(0, 1 - up * 2.4) : 0;
    glowMaterial.color.setRGB(1, 0.72 - low * 0.22, 0.42 - low * 0.3);
    glowMaterial.opacity = day ? 0.1 + low * 0.42 : 0;
    k.sunGlow.visible = day;
    k.sunGlow.scale.setScalar(1100 + low * 1500);

    // Haze picks up whatever the horizon is doing, which is the only reason it
    // reads as distance and not as a smear.
    const hazeColour = new THREE.Color().setRGB(
      0.5 + up * 0.3 + low * 0.4,
      0.5 + up * 0.32 + low * 0.16,
      0.54 + up * 0.34,
    );
    k.haze.forEach((bank, i) => {
      const material = bank.material as THREE.SpriteMaterial;
      material.color
        .copy(hazeColour)
        .multiplyScalar(day ? 1 : 0.22)
        .lerp(new THREE.Color(0x8a_1c_14), bloody * 0.85);
      material.opacity = (0.04 + fog * 0.13 + bloody * 0.03) * (day ? 1 : 0.5) * (1 - i * 0.05);
    });

    // Mist pools in the hollow when the air is wet and the sun is not on it.
    const pooling = Math.min(1, fog * 1.4) * (1 - up * 0.7);
    k.mist.forEach((pool) => {
      const material = pool.material as THREE.SpriteMaterial;
      material.color.copy(hazeColour).multiplyScalar(day ? 0.95 : 0.3);
      material.opacity = pooling * 0.18;
      pool.visible = material.opacity > 0.01;
    });

    k.clouds.forEach((sprite, i) => {
      const share = i / k.clouds.length;
      const material = sprite.material as THREE.SpriteMaterial;
      material.opacity = cloud > share * 0.9 ? Math.min(0.8, 0.1 + cloud * 0.55) : 0;
      const low = day ? Math.max(0, 1 - Math.max(0, elevation) / 22) : 0;
      material.color.setRGB(
        0.34 + low * 0.55 + up * 0.34,
        0.32 + low * 0.24 + up * 0.34,
        0.33 + up * 0.36,
      );
      sprite.visible = material.opacity > 0.01;
    });

    // Snow settles. A white ground is the fastest way to say it is snowing.
    const settled = Math.min(1, snowRate * 1.5);
    k.ground.color.setRGB(0.278 + settled * 0.56, 0.298 + settled * 0.54, 0.204 + settled * 0.6);
    k.foliage.color.setRGB(0.231 + settled * 0.3, 0.322 + settled * 0.26, 0.212 + settled * 0.34);

    const rainMaterial = k.rain.material as THREE.PointsMaterial;
    rainMaterial.opacity = rainRate * 0.6;
    rainMaterial.color.setRGB(0.78, 0.8, 0.85);
    k.rain.visible = rainRate > 0.02;

    const snowMaterial = k.snow.material as THREE.PointsMaterial;
    snowMaterial.opacity = snowRate * 0.8;
    snowMaterial.color.setRGB(1, 1, 1);
    k.snow.visible = snowRate > 0.02;

    k.rainSpeed = 190 + wind * 90;
    k.snowSpeed = 22 + wind * 26;
    k.drift = -wind * 150;

    /*
      Distance haze, tinted to whatever the sky is doing at the horizon.

      Fog the wrong colour is worse than no fog: a grey veil in front of an
      orange sunset reads as a dirty screen. Sampling the horizon means the
      far hills always dissolve into the sky rather than sitting in front of it.
    */
    const horizonColour = new THREE.Color()
      .setRGB(0.42 + up * 0.3, 0.4 + up * 0.32, 0.42 + up * 0.36)
      .lerp(new THREE.Color(0xff_9a_55), Math.max(0, 1 - up * 3) * (day ? 0.55 : 0.1))
      .multiplyScalar(day ? 1 : 0.16)
      .lerp(new THREE.Color(0x3a_08_06), bloody * 0.9);
    k.scene.fog = new THREE.FogExp2(
      horizonColour.getHex(),
      0.00045 + fog * 0.0016 + bloody * 0.0008,
    );
  }, [conditions, hour, dawnHour, daylightHours, hordeTonight]);

  return <div ref={holder} className="absolute inset-0" aria-hidden />;
}

/**
 * A mottled ground texture, drawn rather than shipped.
 *
 * A flat colour stretched over eighteen hundred units reads as a painted
 * backdrop, and there is no near-field detail to tell the eye how far away
 * anything is. Generating it here keeps the panel a single binary with no
 * image assets to embed.
 */
function groundPattern(): THREE.Texture {
  const size = 256;
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = "#8d8f7a";
  ctx.fillRect(0, 0, size, size);
  for (let i = 0; i < 2600; i++) {
    const x = Math.random() * size;
    const y = Math.random() * size;
    const shade = 110 + Math.random() * 90;
    ctx.fillStyle = `rgba(${shade}, ${shade + 8}, ${shade - 24}, ${0.14 + Math.random() * 0.3})`;
    ctx.beginPath();
    ctx.ellipse(x, y, 1 + Math.random() * 7, 1 + Math.random() * 4, Math.random() * 3, 0, 7);
    ctx.fill();
  }
  const texture = new THREE.CanvasTexture(canvas);
  texture.wrapS = texture.wrapT = THREE.RepeatWrapping;
  texture.repeat.set(70, 70);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 4;
  return texture;
}

/** A soft round blob, for cloud billboards, smoke, embers and the moon. */
function blobTexture(): THREE.Texture {
  const size = 128;
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext("2d")!;
  const g = ctx.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
  g.addColorStop(0, "rgba(255,255,255,1)");
  g.addColorStop(0.45, "rgba(255,255,255,0.55)");
  g.addColorStop(1, "rgba(255,255,255,0)");
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, size, size);
  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  return texture;
}
