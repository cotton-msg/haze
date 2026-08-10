import { defineStore } from "pinia"
import { ref } from "vue"
import { api } from "../services/api"
import { wsClient } from "../services/ws"

export interface Call {
  id: string
  caller_id: string
  callee_id: string
  type: "audio" | "video"
  status: "ringing" | "active" | "ended" | "missed" | "rejected"
  started_at: string
}

export const useCallStore = defineStore("call", () => {
  const call = ref<Call | null>(null)
  const direction = ref<"incoming" | "outgoing" | null>(null)
  const peerConnected = ref(false)
  const localStream = ref<MediaStream | null>(null)
  const remoteStream = ref<MediaStream | null>(null)
  const pc = ref<RTCPeerConnection | null>(null)
  const muted = ref(false)
  const videoOff = ref(false)

  // События звонков идут по общему WS-каналу чата (единое соединение).
  const handlers: Array<() => void> = []

  function setup() {
    if (handlers.length) return
    handlers.push(wsClient.on("call_incoming", (p: any) => handleEvent("call_incoming", p)))
    handlers.push(wsClient.on("call_accepted", (p: any) => handleEvent("call_accepted", p)))
    handlers.push(wsClient.on("call_rejected", (p: any) => handleEvent("call_rejected", p)))
    handlers.push(wsClient.on("call_timeout", (p: any) => handleEvent("call_timeout", p)))
    handlers.push(wsClient.on("call_ended", (p: any) => handleEvent("call_ended", p)))
    handlers.push(wsClient.on("webrtc_offer", (p: any) => handleSignal("webrtc_offer", p)))
    handlers.push(wsClient.on("webrtc_answer", (p: any) => handleSignal("webrtc_answer", p)))
    handlers.push(wsClient.on("ice_candidate", (p: any) => handleSignal("ice_candidate", p)))
  }

  function handleEvent(type: string, payload: any) {
    switch (type) {
      case "call_incoming": {
        call.value = payload.call
        direction.value = "incoming"
        break
      }
      case "call_accepted": {
        if (call.value && payload.call && call.value.id === payload.call.id) call.value.status = "active"
        break
      }
      case "call_rejected":
      case "call_timeout":
      case "call_ended": {
        const endedCall = payload.call || (payload.call_id ? { id: payload.call_id } : null)
        if (endedCall && call.value && call.value.id === endedCall.id) {
          cleanup()
        }
        break
      }
    }
  }

  async function handleSignal(type: string, payload: any) {
    const peer = await ensurePeer()
    if (!peer) return
    try {
      if (type === "webrtc_offer") {
        await peer.setRemoteDescription({ type: "offer", sdp: payload.sdp })
        const answer = await peer.createAnswer()
        await peer.setLocalDescription(answer)
        await sendSignal("webrtc_answer", { sdp: answer.sdp })
      } else if (type === "webrtc_answer") {
        await peer.setRemoteDescription({ type: "answer", sdp: payload.sdp })
      } else if (type === "ice_candidate" && payload.candidate) {
        await peer.addIceCandidate(payload.candidate)
      }
    } catch (e) {
      console.warn("signal error", e)
    }
  }

  async function startCall(calleeId: string, kind: "audio" | "video") {
    const res: any = await api.post("/call/start", { callee_id: calleeId, type: kind })
    if (res.error) return
    call.value = res.data
    direction.value = "outgoing"
    await setupLocalMedia(kind)
    await createOffer()
  }

  async function answerCall() {
    if (!call.value) return
    const res: any = await api.post(`/call/${call.value.id}/answer`)
    if (res.error) return
    call.value.status = "active"
    await setupLocalMedia(call.value.type)
    await createOffer()
  }

  async function rejectCall() {
    if (!call.value) return
    await api.post(`/call/${call.value.id}/reject`)
    cleanup()
  }

  async function endCall() {
    if (!call.value) return
    await api.post(`/call/${call.value.id}/end`)
    cleanup()
  }

  async function setupLocalMedia(kind: "audio" | "video") {
    try {
      localStream.value = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: kind === "video",
      })
      const peer = await ensurePeer()
      if (!peer) return
      localStream.value.getTracks().forEach((t) => peer.addTrack(t, localStream.value!))
      localStream.value.getVideoTracks().forEach((t) => (videoOff.value = false))
    } catch (e) {
      console.warn("media error", e)
    }
  }

  async function createOffer() {
    const peer = await ensurePeer()
    if (!peer) return
    const offer = await peer.createOffer()
    await peer.setLocalDescription(offer)
    await sendSignal("webrtc_offer", { sdp: offer.sdp })
  }

  async function ensurePeer(): Promise<RTCPeerConnection | null> {
    if (pc.value) return pc.value
    const iceServers = await fetchIceServers()
    const peer = new RTCPeerConnection({ iceServers })
    pc.value = peer

    peer.onicecandidate = (e) => {
      if (e.candidate) sendSignal("ice_candidate", { candidate: e.candidate })
    }
    peer.ontrack = (e) => {
      if (e.streams?.[0]) remoteStream.value = e.streams[0]
    }
    peer.onconnectionstatechange = () => {
      peerConnected.value = peer.connectionState === "connected"
    }
    return peer
  }

  async function fetchIceServers(): Promise<RTCIceServer[]> {
    try {
      const res: any = await api.get("/call/ice-config")
      if (!res.error && Array.isArray(res.data?.servers)) return res.data.servers
    } catch {
      /* ignore */
    }
    return [
      { urls: "stun:stun.l.google.com:19302" },
      { urls: "stun:stun1.l.google.com:19302" },
    ]
  }

  async function sendSignal(type: string, payload: Record<string, unknown>) {
    if (!call.value) return
    try {
      await api.post(`/call/${call.value.id}/signal`, { type, ...payload })
    } catch {
      /* ignore */
    }
  }

  function toggleMute() {
    muted.value = !muted.value
    localStream.value?.getAudioTracks().forEach((t) => (t.enabled = !muted.value))
  }

  function toggleVideo() {
    videoOff.value = !videoOff.value
    localStream.value?.getVideoTracks().forEach((t) => (t.enabled = !videoOff.value))
  }

  function cleanup() {
    pc.value?.close()
    pc.value = null
    localStream.value?.getTracks().forEach((t) => t.stop())
    localStream.value = null
    remoteStream.value = null
    call.value = null
    direction.value = null
    peerConnected.value = false
  }

  return {
    call,
    direction,
    peerConnected,
    localStream,
    remoteStream,
    muted,
    videoOff,
    setup,
    startCall,
    answerCall,
    rejectCall,
    endCall,
    toggleMute,
    toggleVideo,
    cleanup,
  }
})
