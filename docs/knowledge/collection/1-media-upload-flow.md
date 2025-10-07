# 1. Media Upload Flow

## Overview

This document describes the media upload flow for collection creation, covering file upload to S3/IPFS and metadata storage.

## Sequence Diagram - Synchronous Upload

```mermaid
sequenceDiagram
  autonumber
  actor U as Creator
  participant FE as FE
  participant GQL as GraphQL Gateway
  participant MEDIA as MediaSvc
  participant S3 as S3/MinIO
  participant PIN as Pinata API
  participant MGM as Mongo(media.assets)

  U->>FE: chọn file logo/banner
  FE->>GQL: mutation uploadMedia(files)
  GQL->>MEDIA: Upload (gRPC stream: meta + chunks)
  MEDIA->>S3: putObject(key=media/<sha256>.<ext>)
  MEDIA->>MEDIA: extract mime, width/height, sha256
  alt dedup (sha256 đã tồn tại)
    MEDIA->>MGM: FIND asset by sha256
    MEDIA-->>GQL: {results: [{asset, deduplicated:true}...]}
    GQL-->>FE: media refs (cid/url nếu sẵn có)
  else mới
    MEDIA->>MGM: INSERT asset{ s3Key, sha256, mime, w,h, pin_status="PINNING" }
    MEDIA->>PIN: POST /pinFileToIPFS (multipart stream từ S3 hoặc memory)
    PIN-->>MEDIA: { IpfsHash=CID, PinSize }
    MEDIA->>MGM: UPDATE asset SET ipfs_cid=CID, pin_status="PINNED"
    MEDIA-->>GQL: {results: [{asset(ipfsCid=CID), deduplicated:false}], urls:[gatewayUrl]}
    GQL-->>FE: media refs (logoCid, bannerCid, urls)
  end
```

## Sequence Diagram - Asynchronous Upload

```mermaid
sequenceDiagram
  autonumber
  actor U as Creator
  participant FE as FE
  participant GQL as GraphQL Gateway
  participant MEDIA as MediaSvc
  participant S3 as S3/MinIO
  participant Q as MQ (media.pin.jobs)
  participant W as PinWorker
  participant PIN as Pinata API
  participant MGM as Mongo(media.assets)
  participant WS as GraphQL WS (optional)

  FE->>GQL: uploadMedia(files)
  GQL->>MEDIA: Upload (gRPC stream)
  MEDIA->>S3: putObject(key=media/<sha256>.<ext>)
  MEDIA->>MEDIA: extract mime, w,h, sha256
  MEDIA->>MGM: UPSERT asset{ s3Key, sha256, mime, w,h, pin_status="PENDING" }
  MEDIA->>Q: publish media.pin.request {assetId, s3Key, sha256}
  MEDIA-->>GQL: {asset(pin_status="PENDING"), urls:[cdn s3 (optional)]}
  GQL-->>FE: refs (chưa có CID)

  Note over W: Worker tiêu thụ job
  W->>S3: getObject(s3Key) (stream)
  W->>PIN: POST /pinFileToIPFS (multipart)
  PIN-->>W: { IpfsHash=CID }
  W->>MGM: UPDATE asset SET ipfs_cid=CID, pin_status="PINNED"
  W->>Q: publish media.asset.pinned {assetId, cid, sha256}

  alt nếu bật push WS
    Q-->>WS: fanout onMediaPinned(assetId, cid, gatewayUrl)
    WS-->>FE: realtime update (CID xuất hiện)
  end
```

## Key Components

### Media Service
- Handles multiple file formats (images, videos, audio)
- Generates thumbnails and variants automatically
- Stores original files in S3 or pins to IPFS
- Creates optimized versions for web delivery

### Storage Backend
- **S3**: Primary storage for media files
- **IPFS**: Decentralized storage option
- **CDN**: Content delivery network for fast access
- **Variants**: Multiple sizes/formats for optimization

### Media Processing
- Image resizing and compression
- Video transcoding and thumbnails
- Audio waveform generation
- Format conversion and optimization

## Data Flow

1. **File Selection**: User selects logo and banner files
2. **Upload Request**: Frontend sends files to GraphQL
3. **Processing**: Media service processes and stores files
4. **Storage**: Files saved to S3/IPFS with metadata
5. **Response**: Returns content identifiers and URLs

## File Types Supported

### Images
- **Formats**: PNG, JPG, GIF, WebP, SVG
- **Max Size**: 10MB per file
- **Variants**: Thumbnail, medium, large, original
- **Optimization**: Compression and format conversion

### Videos
- **Formats**: MP4, WebM, MOV
- **Max Size**: 100MB per file
- **Processing**: Thumbnail extraction, compression
- **Streaming**: Adaptive bitrate for large files

### Audio
- **Formats**: MP3, WAV, OGG
- **Max Size**: 50MB per file
- **Processing**: Waveform generation, compression
- **Metadata**: Duration, bitrate, artist info

## Media Metadata Structure

```json
{
  "_id": "media_asset_id",
  "cid": "QmHash...",
  "kind": "image|video|audio",
  "source": "upload|generated",
  "mime": "image/png",
  "bytes": 1024000,
  "width": 1920,
  "height": 1080,
  "s3Key": "collections/logos/...",
  "ipfsCid": "QmHash...",
  "sha256": "hash...",
  "phash": "perceptual_hash",
  "moderation": "approved|pending|rejected",
  "exif": {...},
  "createdAt": "2024-01-01T00:00:00Z"
}
```

## Error Handling

### Upload Failures
- Network interruption during upload
- File size or format validation errors
- Storage backend unavailability
- Retry logic with exponential backoff

### Processing Errors
- Unsupported file formats
- Corrupted file data
- Processing timeout
- Fallback to original file

### Storage Issues
- S3 bucket permissions
- IPFS pinning failures
- CDN synchronization delays
- Backup storage options