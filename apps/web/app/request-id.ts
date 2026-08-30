let requestCounter = 0;

export function createRequestId() {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.getRandomValues === "function") {
    const randomBytes = cryptoApi.getRandomValues(new Uint8Array(16));
    const randomValue = Array.from(randomBytes, (value) => value.toString(16).padStart(2, "0")).join("");
    return `req_${randomValue}`;
  }

  requestCounter += 1;
  return `req_${Date.now().toString(36)}_${requestCounter.toString(36)}`;
}
