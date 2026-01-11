<script lang="ts" setup>
import curseBg from '@/assets/images/curse-breaker-bg-2.jpg'
import curseGlow from '@/assets/images/curse-glow.png'
import cursePortal from '@/assets/images/curse-portal-front.png'
import curseTentacles from '@/assets/images/curse-tentacles.png'

defineProps<{
  showGradient?: boolean
}>()

const bgStyle = `url(${curseBg})`
</script>

<template>
  <div class="curse-background" :class="{ 'curse-background--gradient': showGradient }">
    <img :src="curseGlow" class="curse-background__curse-glow" alt="" />
    <img :src="cursePortal" class="curse-background__curse-portal-front" alt="" />
    <img :src="curseTentacles" class="curse-background__curse-tentacles" alt="" />
    <img :src="curseTentacles" class="curse-background__curse-tentacles-2" alt="" />
  </div>
</template>

<style scoped>
.curse-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-image: v-bind(bgStyle);
  background-position: center;
  background-size: cover;
  z-index: 0;
}

.curse-background > img {
  position: absolute;
}

.curse-background__curse-glow {
  object-fit: cover;
  width: 50%;
  height: 50%;
  right: -150px;
  top: 100px;
  opacity: 0;
  animation: curse-glow-animation 4s ease-in-out infinite;
  transform-origin: center center;
}

.curse-background__curse-portal-front {
  height: 220px;
  right: 55px;
  top: 143px;
  opacity: 1;
  animation: curse-portal-front-animation 4s ease-out infinite;
}

.curse-background__curse-tentacles,
.curse-background__curse-tentacles-2 {
  width: 275px;
  right: 25px;
  top: 70px;
  opacity: 1;
  animation: curse-tentacles-2-animation 4s ease-out infinite;
  transform-origin: right center;
}

.curse-background__curse-tentacles-2 {
  animation-delay: 2s;
}

.curse-background::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: linear-gradient(48deg, #121B25 18.87%, rgba(18, 27, 37, 0.81) 50.18%, rgba(18, 27, 37, 0.47) 67.87%, rgba(18, 27, 37, 0) 84.38%);
  pointer-events: none;
  z-index: 0;
  opacity: 0;
  transition: opacity 0.7s ease;
}

.curse-background--gradient::after {
  opacity: 1;
}

@keyframes curse-glow-animation {
  0% { transform: translate(0); opacity: 0; }
  50% { transform: translate(-1px); opacity: 0.3; }
  100% { transform: translate(0); opacity: 0; }
}

@keyframes curse-portal-front-animation {
  0% { transform: translate(0) scale(1); opacity: 0; }
  50% { opacity: 0.5; }
  100% { transform: translate(-5px) scale(1.1); opacity: 0; }
}

@keyframes curse-tentacles-2-animation {
  0% { transform: scaleX(1); opacity: 0; }
  50% { opacity: 1; }
  100% { transform: scaleX(1.07); opacity: 0; }
}
</style>
