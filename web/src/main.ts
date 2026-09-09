import './style.css'
import Hls from 'hls.js'

const video = document.getElementById('video') as HTMLVideoElement
const overlay = document.getElementById('overlay') as HTMLElement
const status = document.getElementById('status') as HTMLElement
const enter = document.getElementById('enter') as HTMLButtonElement

const LIVE_URL = '/live/index.m3u8'

function ended() {
  status.textContent = 'Transmissão encerrada'
  enter.disabled = true
  video.removeAttribute('src')
  video.load()
}

function attach() {
  if (Hls.isSupported()) {
    const hls = new Hls({
      liveSyncDurationCount: 3,
      maxBufferLength: 12,
      manifestLoadingTimeOut: 10000,
      manifestLoadingMaxRetry: 3,
    })
    hls.loadSource(LIVE_URL)
    hls.attachMedia(video)
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      status.textContent = 'Ao vivo'
      overlay.classList.add('hidden')
      video.play().catch(() => {
        /* playback começa no próximo gesto */
      })
    })
    hls.on(Hls.Events.ERROR, (_evt, data) => {
      if (!data.fatal) return
      switch (data.type) {
        case Hls.ErrorTypes.NETWORK_ERROR:
          hls.startLoad()
          break
        case Hls.ErrorTypes.MEDIA_ERROR:
          hls.recoverMediaError()
          break
        default:
          hls.destroy()
          ended()
      }
    })
  } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
    // iOS Safari: HLS nativo
    video.src = LIVE_URL
    video.addEventListener('loadedmetadata', () => {
      status.textContent = 'Ao vivo'
      overlay.classList.add('hidden')
      video.play().catch(() => {})
    })
  } else {
    status.textContent = 'Seu navegador não suporta HLS'
    enter.disabled = true
  }
}

enter.addEventListener('click', () => {
  enter.disabled = true
  attach()
})
