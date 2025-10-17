import type { ActiveConnectionSession, ActiveSessionCapabilities } from '@/types/connections'

function extractFeaturesFromCapabilities(capabilities?: ActiveSessionCapabilities) {
  if (!capabilities) {
    return undefined
  }
  const features = capabilities.features
  if (features && typeof features === 'object') {
    return features
  }
  return undefined
}

export function sessionSupportsSftp(session?: ActiveConnectionSession | null): boolean {
  if (!session) {
    return false
  }

  const metadata = session.metadata
  if (metadata && typeof metadata === 'object') {
    const metadataRecord = metadata as Record<string, unknown>
    if (Object.prototype.hasOwnProperty.call(metadataRecord, 'sftp_enabled')) {
      const value = metadataRecord.sftp_enabled
      if (value !== undefined) {
        return Boolean(value)
      }
    }
    const metadataCapabilities = metadataRecord.capabilities
    if (
      metadataCapabilities &&
      typeof metadataCapabilities === 'object' &&
      metadataCapabilities !== null
    ) {
      const featuresRecord = extractFeaturesFromCapabilities(
        metadataCapabilities as ActiveSessionCapabilities
      )
      if (featuresRecord && Object.prototype.hasOwnProperty.call(featuresRecord, 'supportsSftp')) {
        return Boolean(featuresRecord.supportsSftp)
      }
    }
  }

  const features = extractFeaturesFromCapabilities(session.capabilities)
  if (features && Object.prototype.hasOwnProperty.call(features, 'supportsSftp')) {
    return Boolean(features.supportsSftp)
  }

  return true
}
