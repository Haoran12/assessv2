const SCORE_DECIMAL_PLACES_KEY = "assessv2_score_decimal_places";
const DEFAULT_SCORE_DECIMAL_PLACES = 2;
const MIN_SCORE_DECIMAL_PLACES = 0;
const MAX_SCORE_DECIMAL_PLACES = 6;

function clampInteger(value: number): number {
  if (!Number.isFinite(value)) {
    return DEFAULT_SCORE_DECIMAL_PLACES;
  }
  const rounded = Math.floor(value);
  if (rounded < MIN_SCORE_DECIMAL_PLACES) {
    return MIN_SCORE_DECIMAL_PLACES;
  }
  if (rounded > MAX_SCORE_DECIMAL_PLACES) {
    return MAX_SCORE_DECIMAL_PLACES;
  }
  return rounded;
}

export function normalizeScoreDecimalPlaces(value: unknown, fallback = DEFAULT_SCORE_DECIMAL_PLACES): number {
  if (typeof value === "number") {
    return clampInteger(value);
  }
  if (typeof value === "string") {
    const parsed = Number(value.trim());
    if (Number.isFinite(parsed)) {
      return clampInteger(parsed);
    }
  }
  return clampInteger(fallback);
}

export function readScoreDecimalPlaces(): number {
  const stored = localStorage.getItem(SCORE_DECIMAL_PLACES_KEY);
  return normalizeScoreDecimalPlaces(stored, DEFAULT_SCORE_DECIMAL_PLACES);
}

export function persistScoreDecimalPlaces(value: unknown): number {
  const normalized = normalizeScoreDecimalPlaces(value, DEFAULT_SCORE_DECIMAL_PLACES);
  localStorage.setItem(SCORE_DECIMAL_PLACES_KEY, String(normalized));
  return normalized;
}

export function toScoreInputStep(decimalPlaces: number): number {
  const normalized = normalizeScoreDecimalPlaces(decimalPlaces, DEFAULT_SCORE_DECIMAL_PLACES);
  return 10 ** (-normalized);
}

export function formatScoreWithDecimalPlaces(value: number | null, decimalPlaces: number): string {
  if (value === null || !Number.isFinite(value)) {
    return "-";
  }
  const normalized = normalizeScoreDecimalPlaces(decimalPlaces, DEFAULT_SCORE_DECIMAL_PLACES);
  return value.toFixed(normalized);
}

export function roundScoreWithDecimalPlaces(value: number, decimalPlaces: number): number {
  const normalized = normalizeScoreDecimalPlaces(decimalPlaces, DEFAULT_SCORE_DECIMAL_PLACES);
  return Number(value.toFixed(normalized));
}
