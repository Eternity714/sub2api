/** Public capability identity. Administrative platform IDs remain unchanged. */
export function publicPlatformId(platform: string): string {
  return platform === 'grsai' || platform === 'toapis' ? 'media' : platform
}

export function publicPlatformLabel(
  platform: string,
  mediaLabel: string = 'Media API',
  format: (value: string) => string = value => value,
): string {
  const identity = publicPlatformId(platform)
  return identity === 'media' ? mediaLabel : format(identity)
}
