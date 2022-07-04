// stores/counter.js
import {defineStore} from 'pinia'

export const useDataStore = defineStore('data', {
    state: () => {
        return {
            directories: [],
            components: [],
        }
    },
    getters: {
        rootDirectory: state => {
            // sort directories by name
            const directories = state.directories.sort((a, b) => {
                return a.name.length - b.name.length
            })[0].name;

            return directories[0]
        },
        allDirectoryStats: state => {
            if (!state.directories.length) return []
            return Object.keys(state.directories[0]).filter(key => key !== 'name')
        },
    },
})