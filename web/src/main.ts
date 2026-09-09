import './style.css'
import Hls from 'hls.js'

const video = document.getElementById('video') as HTMLVideoElement
const loginEl = document.getElementById('login') as HTMLElement
const pwEl = document.getElementById('pw') as HTMLInputElement
const enterBtn = document.getElementById('enter') as HTMLButtonElement
const errEl = document.getElementById('error') as HTMLElement
const resumeBtn = document.getElementById('resume') as HTMLButtonElement
const peersEl = document.getElementById('peers') as HTMLElement

const LIVE_URL = '/live/index.m3u8'
const DEBUG = new URLSearchParams(location.search).has('debug')

declare global {
  interface Window {
    __hls?: Hls | null
  }
}

function showError(msg: string) {
  errEl.textContent = msg
}

function play() {
  video.play().catch(() => {
    resumeBtn.hidden = false
  })
}

function ended() {
  showError('Transmissão encerrada')
  enterBtn.disabled = true
  video.removeAttribute('src')
  video.load()
}

function attach() {
  if (Hls.isSupported()) {
    const hls = new Hls({
      liveSyncDurationCount: 2,
      maxBufferLength: 8,
      manifestLoadingTimeOut: 10000,
      manifestLoadingMaxRetry: 3,
    })
    hls.loadSource(LIVE_URL)
    hls.attachMedia(video)
    if (DEBUG) window.__hls = hls
    hls.on(Hls.Events.MANIFEST_PARSED, play)
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
    video.src = LIVE_URL
    video.addEventListener('loadedmetadata', play)
  } else {
    showError('Seu navegador não suporta HLS')
    enterBtn.disabled = true
  }
}

async function login() {
  showError('')
  enterBtn.disabled = true
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pwEl.value }),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      showError(data.error || 'Falha ao entrar')
      enterBtn.disabled = false
      return
    }
    loginEl.hidden = true
    peersEl.hidden = false
    attach()
  } catch {
    showError('Erro de rede')
    enterBtn.disabled = false
  }
}

function startHeartbeat() {
  setInterval(async () => {
    try {
      const res = await fetch('/api/presence')
      if (res.status === 401) {
        location.reload()
        return
      }
      const data = await res.json()
      peersEl.textContent = `${data.viewers}/${data.max} assistindo`
    } catch {
      /* sem rede; tenta de novo no próximo ciclo */
    }
  }, 10000)
}

enterBtn.addEventListener('click', login)
pwEl.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') login()
})
resumeBtn.addEventListener('click', () => {
  resumeBtn.hidden = true
  video.play().catch(() => {})
})
startHeartbeat()
