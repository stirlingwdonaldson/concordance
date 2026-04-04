"use client";

import { useEffect, useRef, useState } from "react";
import { formatElapsed } from "../lib/format";
import type {
  JobStreamEvent,
  PipelineJob,
  StreamConnectionState,
} from "../lib/types";

export interface UseJobStreamOptions {
  documentId: string;
  wsBase: string;
  onDocumentStatusUpdate: (status: string, progress: number) => void;
  onJobsSnapshot: (jobs: PipelineJob[]) => void;
  onStageChange: () => void;
  onDocumentReady: () => void;
}

export interface JobStreamState {
  streamState: StreamConnectionState;
  streamStatusLabel: string;
  lastEventLabel: string;
  isStreamStale: boolean;
}

export function useJobStream(options: UseJobStreamOptions): JobStreamState {
  const [streamState, setStreamState] = useState<StreamConnectionState>("idle");
  const [lastStreamEventAt, setLastStreamEventAt] = useState<number | null>(
    null,
  );
  const [nowMs, setNowMs] = useState(() => Date.now());

  const { documentId, wsBase } = options;

  const callbacksRef = useRef(options);
  useEffect(() => {
    callbacksRef.current = options;
  });

  useEffect(() => {
    if (lastStreamEventAt === null) {
      return;
    }

    const intervalId = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [lastStreamEventAt]);

  useEffect(() => {
    if (documentId === "") {
      setStreamState("idle");
      setLastStreamEventAt(null);
      return;
    }

    let closed = false;
    let socket: WebSocket | null = null;
    let retryDelayMs = 500;
    let retryTimeout: ReturnType<typeof setTimeout> | null = null;
    let hasConnected = false;

    function handleStreamMessage(rawData: string) {
      let event: JobStreamEvent;
      try {
        event = JSON.parse(rawData) as JobStreamEvent;
      } catch {
        return;
      }

      setLastStreamEventAt(Date.now());
      const callbacks = callbacksRef.current;

      if (event.eventType === "snapshot") {
        const payloadDocument = event.payload.document as
          | { status?: string; progress?: number }
          | undefined;
        if (
          payloadDocument?.status !== undefined &&
          payloadDocument?.progress !== undefined
        ) {
          callbacks.onDocumentStatusUpdate(
            payloadDocument.status,
            payloadDocument.progress,
          );
        }

        const payloadJobs = event.payload.jobs;
        if (Array.isArray(payloadJobs)) {
          callbacks.onJobsSnapshot(payloadJobs as PipelineJob[]);
        }

        return;
      }

      if (event.eventType === "document_status") {
        const status = event.payload.status;
        const progress = event.payload.progress;
        if (typeof status === "string" && typeof progress === "number") {
          callbacks.onDocumentStatusUpdate(status, progress);
          if (status === "ready") {
            callbacks.onDocumentReady();
          }
        }
        return;
      }

      if (
        event.eventType === "stage_started" ||
        event.eventType === "stage_completed" ||
        event.eventType === "stage_failed"
      ) {
        callbacks.onStageChange();
      }
    }

    function connect() {
      if (closed) {
        return;
      }

      setStreamState(hasConnected ? "reconnecting" : "connecting");
      const socketUrl = `${wsBase}/ws/jobs?documentId=${encodeURIComponent(documentId)}`;
      socket = new WebSocket(socketUrl);

      socket.onopen = () => {
        hasConnected = true;
        retryDelayMs = 500;
        setStreamState("connected");
      };

      socket.onmessage = (message) => {
        if (typeof message.data !== "string") {
          return;
        }
        handleStreamMessage(message.data);
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onclose = () => {
        if (closed) {
          return;
        }
        setStreamState("reconnecting");
        retryTimeout = setTimeout(() => {
          connect();
        }, retryDelayMs);
        retryDelayMs = Math.min(retryDelayMs * 2, 10000);
      };
    }

    connect();

    return () => {
      closed = true;
      if (retryTimeout !== null) {
        clearTimeout(retryTimeout);
      }
      socket?.close();
    };
  }, [documentId, wsBase]);

  const streamStatusLabel =
    streamState === "idle"
      ? "Idle"
      : streamState === "connecting"
        ? "Connecting"
        : streamState === "connected"
          ? "Live"
          : "Reconnecting";

  const lastEventLabel =
    lastStreamEventAt === null
      ? "No events yet"
      : `${new Date(lastStreamEventAt).toLocaleTimeString()} (${formatElapsed(nowMs - lastStreamEventAt)})`;

  const isStreamStale =
    streamState === "connected" &&
    lastStreamEventAt !== null &&
    nowMs - lastStreamEventAt > 30000;

  return { streamState, streamStatusLabel, lastEventLabel, isStreamStale };
}
