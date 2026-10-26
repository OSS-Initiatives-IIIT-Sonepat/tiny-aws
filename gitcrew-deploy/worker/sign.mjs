import { createHmac, timingSafeEqual } from "node:crypto";

export function signBody(secret, body) {
  return `sha256=${createHmac("sha256", secret).update(body).digest("hex")}`;
}

export function signaturesMatch(secret, body, header) {
  if (!header) return false;
  const expected = Buffer.from(signBody(secret, body));
  const actual = Buffer.from(header);
  return expected.length === actual.length && timingSafeEqual(expected, actual);
}
