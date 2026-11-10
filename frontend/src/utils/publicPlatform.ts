export function isNativeMediaPlatform(platform: string | null | undefined): boolean {
  return platform === 'grsai' || platform === 'toapis' || platform === 'kuaipao' || platform === 'centos'
}

/** Public capability identity. Administrative platform IDs remain unchanged. */
export function publicPlatformId(platform: string): string {
  return isNativeMediaPlatform(platform) ? 'media' : platform
}

export function publicPlatformLabel(
  platform: string,
  mediaLabel: string = 'Media API',
  format: (value: string) => string = value => value,
): string {
  const identity = publicPlatformId(platform)
  return identity === 'media' ? mediaLabel : format(identity)
}
